import * as assert from 'assert'
import * as os from 'os'
import * as path from 'path'
import * as fs from 'fs'
import {
  Common,
  loadOrderbookContracts,
  OrderProto,
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
  console.log('[PASS] Mainnet OrderAskV2 & OrderBidV2 codehashes verified')

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
  // Flow 5: Sidecar HTTP server health & contracts check
  // -------------------------------------------------------------------------
  const srv = createOrderbookSidecarServer(builder)
  await new Promise<void>((resolve) => srv.listen(0, '127.0.0.1', resolve))
  const port = (srv.address() as any).port
  const axios = require('../src/core-bridge').axios
  const health = await axios.get(`http://127.0.0.1:${port}/internal/ob/health`)
  assert.strictEqual(health.data.ok, true)
  assert.strictEqual(health.data.contracts.tailLen, 150)
  srv.close()
  console.log('[PASS] Flow 5: HTTP sidecar /internal/ob/health verified')

  fs.rmSync(tmpDir, { recursive: true, force: true })
  console.log('\nALL ORDERBOOK SIDECAR TESTS PASSED!')
}

runTests().catch((err) => {
  console.error('TEST FAILED:', err)
  process.exit(1)
})
