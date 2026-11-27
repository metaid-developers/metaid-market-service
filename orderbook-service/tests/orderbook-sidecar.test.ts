import * as assert from 'assert'
import * as os from 'os'
import * as path from 'path'
import * as fs from 'fs'
import {
  Common,
  loadOrderbookContracts,
  OrderProto,
  OrderTokenProto,
  scrypt,
  TokenProto,
} from '../src/core-bridge'
import { InMemoryChainProvider } from '../src/chain-provider'
import { DraftBuildResult, FinalizeResult, getMvcV10TxId, OrderbookTxBuilder, SignRequestItem, UserSignatureInput } from '../src/tx-builder'
import { createOrderbookSidecarServer } from '../src/server'

const { mvc, signTx, toHex } = scrypt
const DUST = OrderProto.DUST_LIMIT

function signRequestsWithPrivKey(reqs: SignRequestItem[], privKey: any): UserSignatureInput[] {
  const pubKeyHex = privKey.publicKey.toBuffer().toString('hex')
  return reqs.map((r) => {
    const tx = new mvc.Transaction(r.txHex)
    const sigObj = signTx(tx, privKey, mvc.Script.fromHex(r.scriptHex), r.satoshis, r.inputIndex, r.sigtype)
    const sigHex = typeof sigObj === 'string' ? sigObj : toHex(sigObj)
    return {
      txIndex: r.txIndex,
      inputIndex: r.inputIndex,
      sig: sigHex,
      publicKey: pubKeyHex,
    }
  })
}

async function completeDraft(builder: OrderbookTxBuilder, draft: DraftBuildResult, privKey: any): Promise<FinalizeResult> {
  let reqs = draft.signRequests
  for (let guard = 0; guard < 5; guard++) {
    const sigs = signRequestsWithPrivKey(reqs, privKey)
    const res = await builder.finalizeAndBroadcast(draft.draftId, sigs, true)
    if (res.status === 'completed') return res
    reqs = res.signRequests || []
  }
  throw new Error(`Draft ${draft.draftId} did not complete within 5 steps`)
}

async function runTests() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ob-sidecar-test-'))
  const contracts = loadOrderbookContracts(tmpDir)

  // Verify mainnet codehashes match Phase 1 compiled outputs
  assert.strictEqual(contracts.askCodeHashHex, '2704a8071b898b895b6d7bac3158959cea17c6d7', 'askCodeHash mismatch')
  assert.strictEqual(contracts.bidCodeHashHex, 'fdf3bc81875c241e4bb58c54fe3272ad6bbccebc', 'bidCodeHash mismatch')
  assert.strictEqual(contracts.tokenOrderCodeHashHex, 'e4082bffecd647405c2a9b3d5f23d7abb2f6018e', 'tokenOrderCodeHash mismatch')
  console.log('[PASS] Mainnet OrderAskV2, OrderBidV2 & OrderTokenV2 codehashes verified')

  // Compute actual local contract check hashes (matching compiled tokenTransferCheck / tokenUnlockContractCheck)
  const tcHashHex = mvc.crypto.Hash.sha256ripemd160(
    Buffer.concat([new contracts.TokenTransferCheck().lockingScript.toBuffer(), Buffer.from('6a', 'hex')])
  ).toString('hex')
  const uc2to5HashHex = mvc.crypto.Hash.sha256ripemd160(
    Buffer.concat([new contracts.TokenUnlockContractCheck().lockingScript.toBuffer(), Buffer.from('6a', 'hex')])
  ).toString('hex')
  const uc4to8HashHex = mvc.crypto.Hash.sha256ripemd160(
    Buffer.concat([new contracts.TokenUnlockContractCheck4to8().lockingScript.toBuffer(), Buffer.from('6a', 'hex')])
  ).toString('hex')

  // Build test contracts using local compiled unlock check hashes so local Token + OrderAskV2 verify 100%
  const localContracts = {
    ...contracts,
    askUnlockCheckHashes: [new scrypt.Bytes(uc2to5HashHex), new scrypt.Bytes(uc4to8HashHex)],
  }
  const transferCheckHashesHex = Array(5).fill(tcHashHex)
  const unlockCheckHashesHex = [uc2to5HashHex, uc4to8HashHex, uc2to5HashHex, uc2to5HashHex, uc2to5HashHex]

  const chain = new InMemoryChainProvider()
  const builder = new OrderbookTxBuilder({
    contracts: localContracts,
    chain,
    net: 'testnet',
    transferCheckHashesHex,
    unlockCheckHashesHex,
  })

  // Test wallets
  const privMaker = new mvc.PrivateKey.fromRandom('testnet')
  const addrMaker = privMaker.toAddress('testnet').toString()
  const privTaker = new mvc.PrivateKey.fromRandom('testnet')
  const addrTaker = privTaker.toAddress('testnet').toString()
  const privFee = new mvc.PrivateKey.fromRandom('testnet')
  const addrFee = privFee.toAddress('testnet').toString()

  // Mint synthetic FT genesis + token UTXOs for Maker and Taker
  const TokenGenesisClass = scrypt.buildContractClass(
    require('../src/core-bridge').loadVerifiedContractDesc('token/tokenGenesis', tmpDir)
  )
  const genesisInst = new TokenGenesisClass()
  genesisInst.setDataPart(
    Common.buildScriptData(
      Buffer.concat([
        Buffer.alloc(40, 0),
        Buffer.alloc(20, 0),
        Buffer.from([8]),
        Buffer.from(new mvc.Address(addrMaker, 'testnet').hashBuffer),
        Buffer.alloc(8, 0),
        Buffer.alloc(20, 0),
        Buffer.alloc(36, 0),
        Common.getUInt32Buf(1),
        Common.getUInt32Buf(1),
        Buffer.from('metacontract'),
      ])
    ).toString('hex')
  )
  const genesisTx = new mvc.Transaction()
  genesisTx.version = Common.TX_VERSION
  genesisTx.addInput(
    new mvc.Transaction.Input({
      output: new mvc.Transaction.Output({ script: mvc.Script.buildPublicKeyHashOut(addrMaker), satoshis: DUST }),
      prevTxId: Common.dummyTxId,
      outputIndex: 0,
      script: mvc.Script.empty(),
    })
  )
  genesisTx.addOutput(new mvc.Transaction.Output({ script: genesisInst.lockingScript, satoshis: DUST }))
  const genesisTxid = chain.registerTx(genesisTx)

  const genesisHash = mvc.crypto.Hash.sha256ripemd160(genesisInst.lockingScript.toBuffer())
  const genesisTxidBuf = Buffer.concat([Common.getTxIdBuf(genesisTxid), Common.getUInt32Buf(0)])

  function mintTokenFor(ownerAddr: string, amount: bigint) {
    const ownerHash = Buffer.from(new mvc.Address(ownerAddr, 'testnet').hashBuffer)
    const tokInst = new contracts.Token(
      transferCheckHashesHex.map((h) => new scrypt.Bytes(h)),
      unlockCheckHashesHex.map((h) => new scrypt.Bytes(h))
    )
    const data = Common.buildScriptData(
      Buffer.concat([
        Buffer.alloc(40, 0),
        Buffer.alloc(20, 0),
        Buffer.from([8]),
        ownerHash,
        Common.getUInt64Buf(amount),
        genesisHash,
        genesisTxidBuf,
        Common.getUInt32Buf(1),
        Common.getUInt32Buf(1),
        Buffer.from('metacontract'),
      ])
    )
    tokInst.setDataPart(data.toString('hex'))
    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    tx.addInput(
      new mvc.Transaction.Input({
        output: new mvc.Transaction.Output({ script: genesisTx.outputs[0].script, satoshis: DUST }),
        prevTxId: genesisTxid,
        outputIndex: 0,
        script: mvc.Script.empty(),
      })
    )
    tx.addOutput(new mvc.Transaction.Output({ script: tokInst.lockingScript, satoshis: DUST }))
    const txid = chain.registerTx(tx)
    const scriptBuf = tokInst.lockingScript.toBuffer()
    return {
      txid,
      vout: 0,
      satoshis: DUST,
      scriptHex: scriptBuf.toString('hex'),
      tokenAmount: amount,
      tokenID: TokenProto.getTokenID(scriptBuf).toString('hex'),
      tokenCodeHash: TokenProto.getScriptCodeHash(scriptBuf).toString('hex'),
    }
  }

  function fundMvc(ownerAddr: string, sats: number) {
    const scriptHex = mvc.Script.buildPublicKeyHashOut(new mvc.Address(ownerAddr, 'testnet')).toHex()
    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    tx.addInput(
      new mvc.Transaction.Input({
        output: new mvc.Transaction.Output({ script: mvc.Script.fromHex(scriptHex), satoshis: sats }),
        prevTxId: Common.dummyTxId,
        outputIndex: 0,
        script: mvc.Script.empty(),
      })
    )
    tx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.fromHex(scriptHex), satoshis: sats }))
    const txid = chain.registerTx(tx)
    return { txid, vout: 0, satoshis: sats, scriptHex }
  }

  const makerTok1 = mintTokenFor(addrMaker, BigInt(10000))
  const tokenID = makerTok1.tokenID
  const tokenCodeHash = makerTok1.tokenCodeHash
  chain.setTokenUtxos(addrMaker, [makerTok1])
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 5_000_000)])

  // -------------------------------------------------------------------------
  // Flow 1: PlaceAsk (10,000 token @ 100 sat/token) -> PartialTakeAsk (4,000) -> FullTakeAsk (6,000)
  // -------------------------------------------------------------------------
  const placeAskDraft = await builder.buildPlaceAsk({
    makerAddress: addrMaker,
    tokenCodeHash,
    tokenID,
    tokenAmount: '10000',
    priceNum: '100',
    priceDen: '1',
    minFillAmount: '1000',
    feeBps: 25,
    feeAddress: addrFee,
  })
  const placeAskRes = await completeDraft(builder, placeAskDraft, privMaker)
  assert.strictEqual(placeAskRes.verified, true)
  assert.strictEqual(placeAskRes.newOrders.length, 1)
  const ask0 = placeAskRes.newOrders[0]
  assert.strictEqual(ask0.remainAmount, '10000')
  console.log('[PASS] Flow 1a: PlaceAsk built, verified, and broadcasted')

  // Partial TakeAsk (buy 4,000 of 10,000 -> continuation Ask with 6,000)
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 5_000_000)])
  const takeAskPartialDraft = await builder.buildTakeAsk({
    takerAddress: addrTaker,
    legs: [
      {
        orderTxid: ask0.txid,
        orderVout: ask0.vout,
        tokenTxid: ask0.tokenTxid!,
        tokenVout: ask0.tokenVout!,
        fillToken: '4000',
      },
    ],
  })
  const takeAskPartialRes = await completeDraft(builder, takeAskPartialDraft, privTaker)
  assert.strictEqual(takeAskPartialRes.verified, true)
  assert.strictEqual(takeAskPartialRes.newOrders.length, 1)
  const ask1 = takeAskPartialRes.newOrders[0]
  assert.strictEqual(ask1.remainAmount, '6000')
  console.log('[PASS] Flow 1b: Partial TakeAsk (0-conf) built & verified (remain 6000)')

  // Full TakeAsk on the 0-conf continuation order (buy remaining 6,000)
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 5_000_000)])
  const takeAskFullDraft = await builder.buildTakeAsk({
    takerAddress: addrTaker,
    legs: [
      {
        orderTxid: ask1.txid,
        orderVout: ask1.vout,
        tokenTxid: ask1.tokenTxid!,
        tokenVout: ask1.tokenVout!,
        fillToken: '6000',
      },
    ],
  })
  const takeAskFullRes = await completeDraft(builder, takeAskFullDraft, privTaker)
  assert.strictEqual(takeAskFullRes.verified, true)
  assert.strictEqual(takeAskFullRes.newOrders.length, 0)
  console.log('[PASS] Flow 1c: Full TakeAsk on 0-conf continuation built & verified')

  // -------------------------------------------------------------------------
  // Flow 2: PlaceAsk -> CancelAsk
  // -------------------------------------------------------------------------
  chain.setTokenUtxos(addrMaker, [mintTokenFor(addrMaker, BigInt(5000))])
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 2_000_000)])
  const placeAsk2 = await builder.buildPlaceAsk({
    makerAddress: addrMaker,
    tokenCodeHash,
    tokenID,
    tokenAmount: '5000',
    priceNum: '120',
    priceDen: '1',
    minFillAmount: '500',
    feeBps: 25,
    feeAddress: addrFee,
  })
  const ask2Res = await completeDraft(builder, placeAsk2, privMaker)
  const ask2 = ask2Res.newOrders[0]
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 2_000_000)])
  const cancelAskDraft = await builder.buildCancel({
    makerAddress: addrMaker,
    side: 'ask',
    orderTxid: ask2.txid,
    orderVout: ask2.vout,
    tokenTxid: ask2.tokenTxid,
    tokenVout: ask2.tokenVout,
  })
  const cancelAskRes = await completeDraft(builder, cancelAskDraft, privMaker)
  assert.strictEqual(cancelAskRes.verified, true)
  console.log('[PASS] Flow 2: CancelAsk built & verified')

  // -------------------------------------------------------------------------
  // Flow 3: PlaceBid (1,000,000 sats @ 100 sat/token) -> PartialTakeBid (4,000) -> FullTakeBid (6,000)
  // -------------------------------------------------------------------------
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 3_000_000)])
  const placeBidDraft = await builder.buildPlaceBid({
    makerAddress: addrMaker,
    tokenCodeHash,
    tokenID,
    spendSats: '1000000',
    priceNum: '100',
    priceDen: '1',
    minFillAmount: '1000',
    feeBps: 25,
    feeAddress: addrFee,
  })
  const placeBidRes = await completeDraft(builder, placeBidDraft, privMaker)
  assert.strictEqual(placeBidRes.verified, true)
  const bid0 = placeBidRes.newOrders[0]
  assert.strictEqual(bid0.remainAmount, '1000000')
  console.log('[PASS] Flow 3a: PlaceBid built & verified')

  // Partial TakeBid (sell 4,000 tokens -> 400,000 sats spent -> continuation Bid 600,000 sats)
  chain.setTokenUtxos(addrTaker, [mintTokenFor(addrTaker, BigInt(15000))])
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 2_000_000)])
  const takeBidPartialDraft = await builder.buildTakeBid({
    takerAddress: addrTaker,
    tokenCodeHash,
    tokenID,
    legs: [{ orderTxid: bid0.txid, orderVout: bid0.vout, fillToken: '4000' }],
  })
  const takeBidPartialRes = await completeDraft(builder, takeBidPartialDraft, privTaker)
  assert.strictEqual(takeBidPartialRes.verified, true)
  assert.strictEqual(takeBidPartialRes.txids.length, 3) // Prep + FillerDeposit + 0-sig Settle
  assert.strictEqual(takeBidPartialRes.newOrders.length, 1)
  const bid1 = takeBidPartialRes.newOrders[0]
  assert.strictEqual(bid1.remainAmount, '600000')
  console.log('[PASS] Flow 3b: Partial TakeBid (Prep+Filler+0-sig Settle) built & verified (remain 600000 sats)')

  // Full TakeBid on 0-conf continuation Bid (sell remaining 6,000 tokens)
  chain.setTokenUtxos(addrTaker, [mintTokenFor(addrTaker, BigInt(6000))])
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 2_000_000)])
  const takeBidFullDraft = await builder.buildTakeBid({
    takerAddress: addrTaker,
    tokenCodeHash,
    tokenID,
    legs: [{ orderTxid: bid1.txid, orderVout: bid1.vout, fillToken: '6000' }],
  })
  const takeBidFullRes = await completeDraft(builder, takeBidFullDraft, privTaker)
  assert.strictEqual(takeBidFullRes.verified, true)
  assert.strictEqual(takeBidFullRes.newOrders.length, 0)
  console.log('[PASS] Flow 3c: Full TakeBid on 0-conf continuation built & verified')

  // -------------------------------------------------------------------------
  // Flow 4: PlaceBid -> CancelBid
  // -------------------------------------------------------------------------
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 1_000_000)])
  const placeBid2 = await builder.buildPlaceBid({
    makerAddress: addrMaker,
    tokenCodeHash,
    tokenID,
    spendSats: '500000',
    priceNum: '100',
    priceDen: '1',
    minFillAmount: '500',
    feeBps: 25,
    feeAddress: addrFee,
  })
  const bid2Res = await completeDraft(builder, placeBid2, privMaker)
  const bid2 = bid2Res.newOrders[0]
  const cancelBidDraft = await builder.buildCancel({
    makerAddress: addrMaker,
    side: 'bid',
    orderTxid: bid2.txid,
    orderVout: bid2.vout,
  })
  const cancelBidRes = await completeDraft(builder, cancelBidDraft, privMaker)
  assert.strictEqual(cancelBidRes.verified, true)
  console.log('[PASS] Flow 4: CancelBid built & verified')

  // -------------------------------------------------------------------------
  // Flow 5: Sidecar HTTP server health, contracts & canonical Go proxy routes
  // -------------------------------------------------------------------------
  const srv = createOrderbookSidecarServer(builder)
  await new Promise<void>((resolve) => srv.listen(0, '127.0.0.1', resolve))
  const port = (srv.address() as any).port
  const baseUrl = `http://127.0.0.1:${port}`
  const axios = require('../src/core-bridge').axios
  const health = await axios.get(`${baseUrl}/internal/ob/health`)
  assert.strictEqual(health.data.ok, true)
  assert.strictEqual(health.data.contracts.tailLen, 150)

  // Verify real HTTP PlaceBid -> Finalize -> CancelBid -> Finalize via canonical Go routes
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 1_000_000)])
  const httpBidPrep = await axios.post(`${baseUrl}/internal/ob/place/bid/prepare`, {
    makerAddress: addrMaker,
    tokenCodeHash,
    tokenID,
    spendSats: '400000',
    priceNum: '100',
    priceDen: '1',
    minFillAmount: '500',
    feeBps: 25,
    feeAddress: addrFee,
  })
  assert.strictEqual(httpBidPrep.data.code, 0)
  const httpBidDraft = httpBidPrep.data.data
  assert.ok(httpBidDraft.draftId)
  const httpBidSigs = signRequestsWithPrivKey(httpBidDraft.signRequests, privMaker)
  const httpBidFin = await axios.post(`${baseUrl}/internal/ob/finalize`, {
    draftId: httpBidDraft.draftId,
    signatures: httpBidSigs,
    broadcast: true,
  })
  assert.strictEqual(httpBidFin.data.code, 0)
  assert.strictEqual(httpBidFin.data.data.status, 'completed')
  assert.strictEqual(httpBidFin.data.data.verified, true)
  const httpBidOrder = httpBidFin.data.data.newOrders[0]
  assert.strictEqual(httpBidOrder.remainAmount, '400000')

  const httpCancelPrep = await axios.post(`${baseUrl}/internal/ob/cancel/prepare`, {
    makerAddress: addrMaker,
    side: 'bid',
    orderTxid: httpBidOrder.txid,
    orderVout: httpBidOrder.vout,
  })
  assert.strictEqual(httpCancelPrep.data.code, 0)
  const httpCancelDraft = httpCancelPrep.data.data
  const httpCancelSigs = signRequestsWithPrivKey(httpCancelDraft.signRequests, privMaker)
  const httpCancelFin = await axios.post(`${baseUrl}/internal/ob/finalize`, {
    draftId: httpCancelDraft.draftId,
    signatures: httpCancelSigs,
    broadcast: true,
  })
  assert.strictEqual(httpCancelFin.data.code, 0)
  assert.strictEqual(httpCancelFin.data.data.status, 'completed')
  assert.strictEqual(httpCancelFin.data.data.verified, true)

  // Verify remaining canonical Go proxy routes are registered (return 400 validation, never 404)
  for (const route of [
    '/internal/ob/place/ask/prepare',
    '/internal/ob/place/token/prepare',
    '/internal/ob/take/ask/prepare',
    '/internal/ob/take/bid/prepare',
    '/internal/ob/take/token/prepare',
  ]) {
    const resp = await axios.post(`${baseUrl}${route}`, {}, { validateStatus: () => true })
    assert.strictEqual(resp.status, 400, `expected 400 validation error on ${route}, got ${resp.status}`)
  }

  // =========================================================================
  // FLOW 6 & 7: FT <-> FT OrderTokenV2 (Place -> Partial Take -> 0-conf Full Take & Maker Cancel)
  // =========================================================================
  const wbtcGenInst = new TokenGenesisClass()
  wbtcGenInst.setDataPart(
    Common.buildScriptData(
      Buffer.concat([
        Buffer.alloc(40, 2),
        Buffer.alloc(20, 2),
        Buffer.from([8]),
        Buffer.from(new mvc.Address(addrTaker, 'testnet').hashBuffer),
        Buffer.alloc(8, 0),
        Buffer.alloc(20, 0),
        Buffer.alloc(36, 0),
        Common.getUInt32Buf(1),
        Common.getUInt32Buf(1),
        Buffer.from('metacontract'),
      ])
    ).toString('hex')
  )
  const wbtcGenTx = new mvc.Transaction()
  wbtcGenTx.version = Common.TX_VERSION
  wbtcGenTx.addInput(
    new mvc.Transaction.Input({
      output: new mvc.Transaction.Output({ script: mvc.Script.buildPublicKeyHashOut(addrTaker), satoshis: DUST }),
      prevTxId: Common.dummyTxId,
      outputIndex: 2,
      script: mvc.Script.empty(),
    })
  )
  wbtcGenTx.addOutput(new mvc.Transaction.Output({ script: wbtcGenInst.lockingScript, satoshis: DUST }))
  const wbtcGenTxid = chain.registerTx(wbtcGenTx)
  const wbtcGenesisHash = mvc.crypto.Hash.sha256ripemd160(wbtcGenInst.lockingScript.toBuffer())
  const wbtcGenesisTxidBuf = Buffer.concat([Common.getTxIdBuf(wbtcGenTxid), Common.getUInt32Buf(0)])

  function mintWbtcFor(ownerAddr: string, amount: bigint) {
    const ownerHash = Buffer.from(new mvc.Address(ownerAddr, 'testnet').hashBuffer)
    const tokInst = new contracts.Token(
      transferCheckHashesHex.map((h) => new scrypt.Bytes(h)),
      unlockCheckHashesHex.map((h) => new scrypt.Bytes(h))
    )
    const data = Common.buildScriptData(
      Buffer.concat([
        Buffer.alloc(40, 2),
        Buffer.alloc(20, 2),
        Buffer.from([8]),
        ownerHash,
        Common.getUInt64Buf(amount),
        wbtcGenesisHash,
        wbtcGenesisTxidBuf,
        Common.getUInt32Buf(1),
        Common.getUInt32Buf(1),
        Buffer.from('metacontract'),
      ])
    )
    tokInst.setDataPart(data.toString('hex'))
    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    tx.addInput(
      new mvc.Transaction.Input({
        output: new mvc.Transaction.Output({ script: wbtcGenInst.lockingScript, satoshis: DUST }),
        prevTxId: wbtcGenTxid,
        outputIndex: 0,
        script: mvc.Script.empty(),
      })
    )
    tx.addOutput(new mvc.Transaction.Output({ script: tokInst.lockingScript, satoshis: DUST }))
    const txid = chain.registerTx(tx)
    const buf = tokInst.lockingScript.toBuffer()
    return {
      txid,
      vout: 0,
      satoshis: DUST,
      scriptHex: buf.toString('hex'),
      tokenAmount: amount,
      tokenCodeHash: TokenProto.getScriptCodeHash(buf).toString('hex'),
      tokenID: TokenProto.getTokenID(buf).toString('hex'),
    }
  }

  const makerMcForTok = mintTokenFor(addrMaker, BigInt(1000))
  const takerWbtc1 = mintWbtcFor(addrTaker, BigInt(1000000))
  const makerWbtc1 = mintWbtcFor(addrMaker, BigInt(300000))
  const wbtcMeta = { codeHash: takerWbtc1.tokenCodeHash, tokenID: takerWbtc1.tokenID }
  chain.setTokenUtxos(addrMaker, [makerMcForTok, makerWbtc1])
  chain.setTokenUtxos(addrTaker, [takerWbtc1])
  chain.setMvcUtxos(addrMaker, [fundMvc(addrMaker, 5_000_000)])
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 5_000_000)])

  // Flow 6: Maker sells 1000 MC for WBTC (500 WBTC per MC) -> Taker partial fills 400 -> 0-conf full fills 600
  const tokPlaceDraft = await builder.buildPlaceTokenOrder({
    makerAddress: addrMaker,
    sellTokenCodeHash: tokenCodeHash,
    sellTokenID: tokenID,
    buyTokenCodeHash: wbtcMeta.codeHash,
    buyTokenID: wbtcMeta.tokenID,
    sellAmount: '1000',
    priceNum: '500',
    priceDen: '1',
    minFillAmount: '200',
    feeBps: 25,
    feeAddress: addrFee,
  })
  assert.ok(
    tokPlaceDraft.signRequests.every((r) => r.address === addrMaker),
    'default path (no fundingAddress): all sign requests must be keyed to makerAddress'
  )
  const tokPlaceRes = await completeDraft(builder, tokPlaceDraft, privMaker)
  assert.strictEqual(tokPlaceRes.verified, true)
  assert.strictEqual(tokPlaceRes.newOrders.length, 1)
  const tokOrder1 = tokPlaceRes.newOrders[0]
  assert.strictEqual(tokOrder1.side, 'token')
  assert.strictEqual(tokOrder1.remainAmount, '1000')

  const tokTake1Draft = await builder.buildTakeTokenOrder({
    takerAddress: addrTaker,
    buyTokenCodeHash: wbtcMeta.codeHash,
    buyTokenID: wbtcMeta.tokenID,
    legs: [
      {
        orderTxid: tokOrder1.txid,
        orderVout: tokOrder1.vout,
        tokenTxid: tokOrder1.tokenTxid!,
        tokenVout: tokOrder1.tokenVout!,
        fillSellAmount: '400',
      },
    ],
  })
  const tokTake1Res = await completeDraft(builder, tokTake1Draft, privTaker)
  assert.strictEqual(tokTake1Res.verified, true)
  assert.strictEqual(tokTake1Res.txids.length, 3) // PrepTx + DepositTx + 0-sig SettleTx
  assert.strictEqual(tokTake1Res.newOrders.length, 1)
  const tokOrder2 = tokTake1Res.newOrders[0]
  assert.strictEqual(tokOrder2.remainAmount, '600')

  // 0-conf Full Take on continuation OrderTokenV2!
  const takerWbtc2 = mintWbtcFor(addrTaker, BigInt(1000000))
  chain.setTokenUtxos(addrTaker, [takerWbtc2])
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 5_000_000)])
  const tokTake2Draft = await builder.buildTakeTokenOrder({
    takerAddress: addrTaker,
    buyTokenCodeHash: wbtcMeta.codeHash,
    buyTokenID: wbtcMeta.tokenID,
    legs: [
      {
        orderTxid: tokOrder2.txid,
        orderVout: tokOrder2.vout,
        tokenTxid: tokOrder2.tokenTxid!,
        tokenVout: tokOrder2.tokenVout!,
        fillSellAmount: '600',
      },
    ],
  })
  const tokTake2Res = await completeDraft(builder, tokTake2Draft, privTaker)
  assert.strictEqual(tokTake2Res.verified, true)
  assert.strictEqual(tokTake2Res.newOrders.length, 0)
  console.log('[PASS] Flow 6: FT<->FT Place -> Partial Take -> 0-conf Full Take via OrderTokenFiller verified')

  // Flow 7: Maker places WBTC->MC order via HTTP & cancels via HTTP /internal/ob/cancel/prepare (side: token)
  const httpTokPrep = await axios.post(`${baseUrl}/internal/ob/place/token/prepare`, {
    makerAddress: addrMaker,
    sellTokenCodeHash: wbtcMeta.codeHash,
    sellTokenID: wbtcMeta.tokenID,
    buyTokenCodeHash: tokenCodeHash,
    buyTokenID: tokenID,
    sellAmount: '200000',
    priceNum: '1',
    priceDen: '500',
    minFillAmount: '50000',
    feeBps: 0,
  })
  assert.strictEqual(httpTokPrep.data.code, 0)
  let tokDraft = httpTokPrep.data.data
  let tokFinData: any = null
  for (let s = 0; s < 3; s++) {
    const sigs = signRequestsWithPrivKey(tokDraft.signRequests, privMaker)
    const fin = await axios.post(`${baseUrl}/internal/ob/finalize`, {
      draftId: tokDraft.draftId,
      signatures: sigs,
      broadcast: true,
    })
    assert.strictEqual(fin.data.code, 0)
    tokFinData = fin.data.data
    if (tokFinData.status === 'completed') break
    tokDraft = tokFinData
  }
  assert.strictEqual(tokFinData.status, 'completed')
  const tokCancelTarget = tokFinData.newOrders[0]

  const httpTokCancelPrep = await axios.post(`${baseUrl}/internal/ob/cancel/prepare`, {
    makerAddress: addrMaker,
    side: 'token',
    orderTxid: tokCancelTarget.txid,
    orderVout: tokCancelTarget.vout,
    tokenTxid: tokCancelTarget.tokenTxid,
    tokenVout: tokCancelTarget.tokenVout,
  })
  assert.strictEqual(httpTokCancelPrep.data.code, 0)
  let cDraft = httpTokCancelPrep.data.data
  let cFinData: any = null
  for (let s = 0; s < 3; s++) {
    const sigs = signRequestsWithPrivKey(cDraft.signRequests, privMaker)
    const fin = await axios.post(`${baseUrl}/internal/ob/finalize`, {
      draftId: cDraft.draftId,
      signatures: sigs,
      broadcast: true,
    })
    assert.strictEqual(fin.data.code, 0)
    cFinData = fin.data.data
    if (cFinData.status === 'completed') break
    cDraft = cFinData
  }
  assert.strictEqual(cFinData.status, 'completed')
  assert.strictEqual(cFinData.verified, true)
  console.log('[PASS] Flow 7: FT<->FT HTTP Place & Maker Cancel verified')


  // =========================================================================
  // FLOW 8: Burn-maker floor bid funded by a separate fundingAddress
  // (lock WBTC -> buy MC; makerAddr = zero hash; uncancellable by design)
  // =========================================================================
  const burnAddr = mvc.Address.fromPublicKeyHash(Buffer.alloc(20, 0), 'testnet').toString()
  const burnHash20 = Buffer.alloc(20, 0)
  const privFunder = new mvc.PrivateKey.fromRandom('testnet')
  const addrFunder = privFunder.toAddress('testnet').toString()
  const funderP2pkhHex = mvc.Script.buildPublicKeyHashOut(new mvc.Address(addrFunder, 'testnet')).toHex()
  const burnP2pkhHex = mvc.Script.buildPublicKeyHashOut(new mvc.Address(burnAddr, 'testnet')).toHex()

  async function assertThrowsErr(fn: () => Promise<any>, re: RegExp, label: string) {
    try {
      await fn()
    } catch (err) {
      assert.ok(re.test(String(err?.message || err)), label + ': unexpected error message: ' + String(err?.message || err))
      console.log('[PASS] ' + label)
      return
    }
    throw new Error(label + ': expected error was not thrown')
  }

  const burnBidParams = {
    makerAddress: burnAddr,
    fundingAddress: addrFunder,
    sellTokenCodeHash: wbtcMeta.codeHash,
    sellTokenID: wbtcMeta.tokenID,
    buyTokenCodeHash: tokenCodeHash,
    buyTokenID: tokenID,
    sellAmount: '200000',
    priceNum: '1',
    priceDen: '500',
    minFillAmount: '50000',
    feeBps: 0,
    feeAddress: '',
  }

  // --- rejections ---
  await assertThrowsErr(
    () => builder.buildPlaceTokenOrder({ ...burnBidParams, fundingAddress: 'not-a-valid-address' }),
    /fundingAddress is not a valid/,
    'Flow 8a: invalid fundingAddress rejected'
  )
  await assertThrowsErr(
    () => builder.buildPlaceTokenOrder({ ...burnBidParams, fundingAddress: burnAddr }),
    /fundingAddress cannot be the burn/,
    'Flow 8b: burn fundingAddress rejected'
  )
  await assertThrowsErr(
    () => builder.buildPlaceTokenOrder({ ...burnBidParams, fundingAddress: undefined }),
    /makerAddress cannot be the burn/,
    'Flow 8c: burn makerAddress without fundingAddress rejected'
  )

  // --- fund the funder with WBTC + SPACE, spy on UTXO lookups ---
  const funderWbtc = mintWbtcFor(addrFunder, BigInt(300000))
  chain.setTokenUtxos(addrFunder, [funderWbtc])
  chain.setMvcUtxos(addrFunder, [fundMvc(addrFunder, 5_000_000)])

  const origGetTokenUtxos = chain.getTokenUtxos.bind(chain)
  const origGetMvcUtxos = chain.getMvcUtxos.bind(chain)
  const queriedTokenAddrs: string[] = []
  const queriedMvcAddrs: string[] = []
  ;(chain as any).getTokenUtxos = async (a: string, ch: string, id: string) => {
    queriedTokenAddrs.push(a)
    return origGetTokenUtxos(a, ch, id)
  }
  ;(chain as any).getMvcUtxos = async (a: string) => {
    queriedMvcAddrs.push(a)
    return origGetMvcUtxos(a)
  }

  const burnDraft = await builder.buildPlaceTokenOrder({ ...burnBidParams })

  ;(chain as any).getTokenUtxos = origGetTokenUtxos
  ;(chain as any).getMvcUtxos = origGetMvcUtxos

  assert.ok(queriedTokenAddrs.length > 0 && queriedTokenAddrs.every((a) => a === addrFunder), 'token UTXO lookups must hit fundingAddress only')
  assert.ok(queriedMvcAddrs.length > 0 && queriedMvcAddrs.every((a) => a === addrFunder), 'SPACE UTXO lookups must hit fundingAddress only')
  console.log('[PASS] Flow 8d: UTXO lookups hit fundingAddress only')

  // --- drive both steps with the funder key, collecting all sign requests ---
  const allBurnReqs: SignRequestItem[] = [...burnDraft.signRequests]
  let burnRes: FinalizeResult | null = null
  let burnReqs = burnDraft.signRequests
  for (let g = 0; g < 5; g++) {
    const sigs = signRequestsWithPrivKey(burnReqs, privFunder)
    const res = await builder.finalizeAndBroadcast(burnDraft.draftId, sigs, true)
    if (res.status === 'completed') {
      burnRes = res
      break
    }
    burnReqs = res.signRequests || []
    allBurnReqs.push(...burnReqs)
  }
  assert.ok(burnRes, 'burn-maker place draft did not complete')
  assert.strictEqual(burnRes!.verified, true)
  assert.ok(allBurnReqs.length > 0 && allBurnReqs.every((r) => r.address === addrFunder), 'every sign request (step0+step1) must be keyed to fundingAddress')
  console.log('[PASS] Flow 8e: 2-step burn-maker place completed with funder key; all sign requests keyed to fundingAddress')

  // --- change outputs go to funder, never to burn ---
  const burnPrepTx = new mvc.Transaction(burnRes!.txHexes[0])
  const burnLockTx = new mvc.Transaction(burnRes!.txHexes[1])
  assert.strictEqual(burnPrepTx.outputs[2].script.toHex(), funderP2pkhHex, 'prep vout2 (lock-fee UTXO) must pay fundingAddress')
  assert.ok(
    burnPrepTx.outputs.every((o: any) => o.script.toHex() !== burnP2pkhHex) &&
      burnLockTx.outputs.every((o: any) => o.script.toHex() !== burnP2pkhHex),
    'no output may pay the burn address'
  )
  // token change (300000 - 200000 = 100000 WBTC) locked to funder hash20
  const tokChangeBuf = burnLockTx.outputs[1].script.toBuffer()
  assert.ok(TokenProto.getTokenAddress(tokChangeBuf).equals(Buffer.from(new mvc.Address(addrFunder, 'testnet').hashBuffer)), 'token change must go to fundingAddress hash20')
  assert.strictEqual(TokenProto.getTokenAmount(tokChangeBuf).toString(), '100000')
  console.log('[PASS] Flow 8f: prep/lock change outputs pay fundingAddress, token change locked to funder')

  // --- order tail: makerAddr zero hash, zero fee, requested economics ---
  const burnOrder = burnRes!.newOrders[0]
  const burnTail = OrderTokenProto.parseOrderTokenScript(Buffer.from(burnOrder.scriptHex, 'hex'))
  assert.ok(burnTail.makerAddr.equals(burnHash20), 'order tail makerAddr must be 20 zero bytes')
  assert.ok(burnTail.feeAddr.equals(OrderTokenProto.NULL_ADDRESS), 'feeAddr must be NULL_ADDRESS')
  assert.strictEqual(burnTail.feeBps, 0)
  assert.strictEqual(burnTail.remainAmount.toString(), '200000')
  assert.strictEqual(burnTail.minFillAmount.toString(), '50000')
  assert.strictEqual(burnTail.priceNum.toString(), '1')
  assert.strictEqual(burnTail.priceDen.toString(), '500')
  assert.strictEqual(burnTail.sellTokenID.toString('hex'), wbtcMeta.tokenID)
  assert.strictEqual(burnTail.buyTokenID.toString('hex'), tokenID)
  console.log('[PASS] Flow 8g: order tail makerAddr = zero hash, zero-fee NULL feeAddr, economics as requested')

  // --- taker partially fills the burn bid: pays 100 MC, receives 50000 WBTC ---
  const takerMc = mintTokenFor(addrTaker, BigInt(500))
  chain.setTokenUtxos(addrTaker, [takerMc])
  chain.setMvcUtxos(addrTaker, [fundMvc(addrTaker, 5_000_000)])
  const burnTakeDraft = await builder.buildTakeTokenOrder({
    takerAddress: addrTaker,
    buyTokenCodeHash: tokenCodeHash,
    buyTokenID: tokenID,
    legs: [
      {
        orderTxid: burnOrder.txid,
        orderVout: burnOrder.vout,
        tokenTxid: burnOrder.tokenTxid!,
        tokenVout: burnOrder.tokenVout!,
        fillSellAmount: '50000',
      },
    ],
  })
  const burnTakeRes = await completeDraft(builder, burnTakeDraft, privTaker)
  assert.strictEqual(burnTakeRes.verified, true)
  assert.strictEqual(burnTakeRes.newOrders.length, 1)
  assert.strictEqual(burnTakeRes.newOrders[0].remainAmount, '150000')

  const settleTx = new mvc.Transaction(burnTakeRes.txHexes[burnTakeRes.txHexes.length - 1])
  const settleScripts = settleTx.outputs.map((o: any) => o.script.toBuffer())
  // bought MC (ceil(50000 * 1 / 500) = 100, zero fee) locked to the zero hash
  const mcSampleBuf = Buffer.from(takerMc.scriptHex, 'hex')
  const expectedBurnMc = TokenProto.getNewTokenScript(mcSampleBuf, burnHash20, BigInt(100))
  assert.ok(
    settleScripts.some((s: Buffer) => s.equals(Buffer.from(expectedBurnMc))),
    'settle tx must contain the bought MC output locked to the zero/burn hash'
  )
  // taker receives the 50000 WBTC
  const lockedWbtcSample = burnLockTx.outputs[0].script.toBuffer()
  const takerHash20 = Buffer.from(new mvc.Address(addrTaker, 'testnet').hashBuffer)
  const expectedTakerWbtc = TokenProto.getNewTokenScript(lockedWbtcSample, takerHash20, BigInt(50000))
  assert.ok(
    settleScripts.some((s: Buffer) => s.equals(Buffer.from(expectedTakerWbtc))),
    'settle tx must contain the 50000 WBTC output locked to the taker'
  )
  console.log('[PASS] Flow 8h: take-token against burn-maker bid: taker receives WBTC, bought MC locked to zero hash, remain updated')

  // --- cancel of the burn-maker continuation order must fail explicitly ---
  const burnContinuation = burnTakeRes.newOrders[0]
  await assertThrowsErr(
    () =>
      builder.buildCancel({
        makerAddress: burnAddr,
        side: 'token',
        orderTxid: burnContinuation.txid,
        orderVout: burnContinuation.vout,
        tokenTxid: burnContinuation.tokenTxid,
        tokenVout: burnContinuation.tokenVout,
      }),
    /uncancellable by design/,
    'Flow 8i: cancel of burn-maker order fails with explicit error'
  )

  srv.close()
  console.log('[PASS] Flow 5: HTTP sidecar health & canonical Go proxy routes verified')

  fs.rmSync(tmpDir, { recursive: true, force: true })
  console.log('\nALL ORDERBOOK SIDECAR TESTS PASSED!')
}

runTests().catch((err) => {
  console.error('TEST FAILED:', err)
  process.exit(1)
})
