# ID-Coin 铸造流动性 → MVC WBTC 兜底买单（Floor Bid）规格

状态：设计已确认（2026-11-28）。owner 决策：兜底单撤不掉 + 回购币销毁；兜底价扣减 1.5%（含过桥与矿工成本）；老币历史缺口暂不补（后续用自有 WBTC 处理）；新发行最低 liquidityPerMint 为可选配置；FT/FT 订单簿复用 orderbook 团队的 `OrderTokenV2`，本项目不自建 FT/FT。

## 1. 端到端流程

```text
用户在 BTC 上铸造 ID-Coin（market-service 自建两段 reveal：follow pin → mint）
  └─ mint 交易必须付款到部署铭文写死的 payTo（MAN 索引器校验，不可改）
       ├─ 老币（32 个快照币）：payTo = 旧池地址（私钥我方持有但非独占）
       │     → idcoin-keeper 发现入账即刻转到该币的 LP 金库（转账矿工费由铸造用户在 payTo 金额上额外支付）
       └─ 新币：payTo = 该币的 LP 金库（新独占助记词派生）
LP 金库（每币一个）按币攒批，满足成本预算后：
  → octopus btc_mint（keeper 作为普通客户端：createPrepayOrderMintBtc / submitPrepayOrderMintBtc）
  → WBTC 1:1 铸到 keeper 的 MVC LP 资金地址
  → keeper 用本地 orderbook tx-builder 构造 OrderTokenV2 订单并签名：
       sellToken = WBTC（锁定），buyToken = 该币的桥接 MVC FT，
       makerAddr = 销毁地址 1111111111111111111114oLvT2（hash160 全 0 → 无法撤单，回购的币即销毁），
       fundingAddress = MVC LP 资金地址，feeBps = 0，无 feeAddress
持币人：ID-Coin MRC20 → 资产桥 mrc20_mint → MVC FT → 卖给兜底单拿 WBTC → btc_redeem 回 BTC
```

## 2. 密钥与派生（全部私钥只在 47.242.162.36 的 secrets/ 下，root 0400）

| 用途 | 来源 | 派生 / 说明 |
|---|---|---|
| 新币 payTo / 所有币 LP 金库（BTC） | `secrets/idcoin_lp_mnemonic.txt` | `m/84'/0'/1000'/<UPPER(tick) 的 UTF-8 字节，逐字节非硬化>`，P2WPKH。账户 xpub 供 market-service 离线算地址（配置项 `idcoins.lp_vault_xpub`，不入公开仓库） |
| MVC LP 资金地址（收 WBTC、付挂单 SPACE 手续费） | 同上 | `m/44'/10001'/1000'/0/0`，P2PKH：`1AxqcM6yFDbAsYu3LehFQTHRTK321KRt8m` |
| 老币旧池（32 个 payTo） | `secrets/idcoin_legacy_pool_keys.json` | `{version, coins: {tickId: {tick, address, source, wif}}}`；12 个源自 w-node 单签（`m/<"idCoins"+tick+"1" 字节>`），20 个源自 w-sign-service 明文 ks；生成时逐个校验 WIF→地址 = 链上 payTo |

样例：SUNNY LP 金库 `bc1qg0lt0nc4uja6fyhp8f0n2lfupd3hxld3vfeayf`，METAID LP 金库 `bc1qlz9dfj0d3p8jl8870sdyhlnryhc9e7t37d3djd`。

## 3. 兜底价与批次

- 兜底价（sats/币）= `liquidityPerMint × (1 − 1.5%) / amtPerMint`。OrderTokenV2 原始单位：`priceNum/priceDen` = buyToken（币 FT 原始单位）/ sellToken（WBTC sats）= `amtPerMint·10^decimals·1000 / (liquidityPerMint·985)`，约分后须在 uint64 内；合约对 buyAmount 向上取整（对销毁方有利）。
- 成本预算：每批 `存款交易矿工费 + 桥服务费 max(1000, 320×feeRate)` ≤ 批内金额 × 1.5% 才发批；否则继续累积（不降低兜底价）。单笔 btc_mint 限 10,000 ~ 100,000,000 sats，超限拆批；>100k sats 需 2 确认。
- 每批对应一张兜底单；同价多单在盘口聚合。实际成本低于预算的部分全部进兜底单（多出的深度惠及老币持有人）。

## 4. 安全不变量（keeper 签名前必须本地校验）

1. 旧池转账只能到该币 LP 金库；LP 金库出款只能到桥 BTC 金库（`bc1qh90z428z4s0g9prcdf6rq00yg27uhxrtxlvvvv`）与桥返回且在白名单内的手续费地址，金额与订单一致。
2. MVC 侧只签本地 tx-builder 构造的挂单交易：尾部 sellToken=WBTC（codehash `c9cc7bbd1010b44873959a8b1a2bcedeb62302b7`，tokenID `2ee1af1ae2d3605f08894c17358c99f1fc4c1325`）、buyToken=该币桥接 FT、makerAddr=全 0、feeBps=0、价格与计算值一致；找零只回 LP 资金地址。
3. 默认 dry-run；全局与按币的开关；所有状态落库可重放、幂等。

## 5. 分期

| 期 | 内容 | 依赖 |
|---|---|---|
| P1a | orderbook sidecar：`fundingAddress` 与 `makerAddr` 分离（允许销毁地址做 maker） | — |
| P1b | idcoin-keeper：密钥加载、币注册表、老池即到即转、LP 金库账本、dry-run | 密钥文件（已就绪） |
| P2 | keeper：攒批 → btc_mint → WBTC 到账 → 本地构造并广播兜底单 | 桥上注册各币 MRC20↔FT 资产对（需桥管理员签名 + SPACE 燃料）；FT/FT 订单簿与 Higun 索引上线 |
| P3 | market-service 自建 ID-Coin 铸造（两段 reveal），老币 payTo 额外加转账费；按币开关逐个放开 | P1b 上线 |
| P4 | 自建发行（Launch）：payTo 用 xpub 派生、tickSign、CheckIdCoins 多公钥、DB 驱动的币集合、可选最低 liquidityPerMint | P3 |
| P5 | coins-info `ordersPool/ordersPrice` 改为兜底单深度/价格；前端铸造与「卖给兜底单」入口 | P2 |
