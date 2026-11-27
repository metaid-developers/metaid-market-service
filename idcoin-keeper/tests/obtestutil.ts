/**
 * Test harness building REAL OrderTokenV2 place drafts with the orderbook
 * tx-builder + InMemoryChainProvider (synthetic FT genesis, no network).
 * Mirrors orderbook-service/tests/orderbook-sidecar.test.ts setup.
 */
import * as fs from 'fs'
import * as os from 'os'
import * as path from 'path'
import {
  Common,
  loadOrderbookContracts,
  OrderProto,
  scrypt,
  TokenProto,
} from '../../orderbook-service/src/core-bridge'
import { InMemoryChainProvider } from '../../orderbook-service/src/chain-provider'
import { OrderbookTxBuilder, DraftBuildResult } from '../../orderbook-service/src/tx-builder'
import { floorBidParams, BidExpectation, WBTC_CODEHASH, WBTC_TOKEN_ID } from '../src/bid'

const { mvc } = scrypt
const DUST = OrderProto.DUST_LIMIT

export async function makeHarness() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'obtest-'))
  const contracts = loadOrderbookContracts(tmpDir)
  const tcHashHex = mvc.crypto.Hash.sha256ripemd160(
    Buffer.concat([new contracts.TokenTransferCheck().lockingScript.toBuffer(), Buffer.from('6a', 'hex')])
  ).toString('hex')
  const uc2to5HashHex = mvc.crypto.Hash.sha256ripemd160(
    Buffer.concat([new contracts.TokenUnlockContractCheck().lockingScript.toBuffer(), Buffer.from('6a', 'hex')])
  ).toString('hex')
  const uc4to8HashHex = mvc.crypto.Hash.sha256ripemd160(
    Buffer.concat([new contracts.TokenUnlockContractCheck4to8().lockingScript.toBuffer(), Buffer.from('6a', 'hex')])
  ).toString('hex')
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

  const privFunding = new (mvc.PrivateKey as any).fromRandom('testnet')
  const addrFunding = privFunding.toAddress('testnet').toString()

  // synthetic FT genesis (stands in for both WBTC and the coin FT)
  const TokenGenesisClass = scrypt.buildContractClass(
    require('../../orderbook-service/src/core-bridge').loadVerifiedContractDesc('token/tokenGenesis', tmpDir)
  )
  const genesisInst = new TokenGenesisClass()
  genesisInst.setDataPart(
    Common.buildScriptData(
      Buffer.concat([
        Buffer.alloc(40, 0),
        Buffer.alloc(20, 0),
        Buffer.from([8]),
        Buffer.from(new mvc.Address(addrFunding).hashBuffer),
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
      output: new mvc.Transaction.Output({ script: mvc.Script.buildPublicKeyHashOut(addrFunding), satoshis: DUST }),
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
    const ownerHash = Buffer.from(new mvc.Address(ownerAddr).hashBuffer)
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
    const scriptHex = mvc.Script.buildPublicKeyHashOut(new mvc.Address(ownerAddr)).toHex()
    const tx = new mvc.Transaction()
    tx.version = Common.TX_VERSION
    tx.addInput(
      new mvc.Transaction.Input({
        output: new mvc.Transaction.Output({ script: mvc.Script.buildPublicKeyHashOut(ownerAddr), satoshis: sats }),
        prevTxId: Common.dummyTxId,
        outputIndex: 0,
        script: mvc.Script.empty(),
      })
    )
    tx.addOutput(new mvc.Transaction.Output({ script: mvc.Script.buildPublicKeyHashOut(ownerAddr), satoshis: sats }))
    const txid = chain.registerTx(tx)
    return { txid, vout: 0, satoshis: sats, scriptHex }
  }

  const sellAmount = 200000n
  const priceNum = 1000000000n
  const priceDen = 197n

  async function buildValidDraft(): Promise<DraftBuildResult> {
    // funding owns "WBTC" tokens + SPACE; in tests the token ids are synthetic —
    // validation compares against the SAME ids, exercising the full path.
    const wbtc = mintTokenFor(addrFunding, sellAmount)
    const space = fundMvc(addrFunding, 100000)
    chain.setTokenUtxos(addrFunding, [wbtc])
    chain.setMvcUtxos(addrFunding, [space])
    const params = {
      makerAddress: 'mfWxJ45yp2SFn7UciZyNpvDKrzbhyfKrY8', // testnet zero-hash burn
      fundingAddress: addrFunding,
      sellTokenCodeHash: wbtc.tokenCodeHash,
      sellTokenID: wbtc.tokenID,
      buyTokenCodeHash: 'cc'.repeat(20),
      buyTokenID: 'dd'.repeat(20),
      sellAmount: sellAmount.toString(),
      priceNum: priceNum.toString(),
      priceDen: priceDen.toString(),
      minFillAmount: '1000',
      feeBps: 0,
      feeAddress: '',
    }
    const draft = await (builder as any).buildPlaceTokenOrder(params)
    ;(draft as any).__params = params
    return draft
  }

  function expectationFor(draft: any): BidExpectation {
    const p = draft.__params
    return {
      fundingAddress: addrFunding,
      sellTokenCodeHash: p.sellTokenCodeHash,
      sellTokenID: p.sellTokenID,
      buyTokenCodeHash: p.buyTokenCodeHash,
      buyTokenID: p.buyTokenID,
      sellAmount,
      priceNum,
      priceDen,
      minFillAmount: 1000n,
    }
  }

  function craftForeignOutputDraft(draft: DraftBuildResult): DraftBuildResult {
    // clone the draft but graft a foreign P2PKH output into the first tx
    const clone: DraftBuildResult = JSON.parse(JSON.stringify(draft))
    const req = clone.signRequests[0]
    const tx = new mvc.Transaction(req.txHex)
    const foreign = new (mvc.PrivateKey as any).fromRandom('testnet').toAddress('testnet')
    tx.addOutput(new mvc.Transaction.Output({
      script: mvc.Script.buildPublicKeyHashOut(foreign),
      satoshis: 1000,
    }))
    req.txHex = tx.uncheckedSerialize()
    return clone
  }

  return { builder, chain, contracts, privFunding, addrFunding, buildValidDraft, craftForeignOutputDraft, expectationFor }
}
