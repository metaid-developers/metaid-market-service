# 清除 orders.exchange 依赖（前端 + 后端）

## 默认决策（未经用户确认，按推荐执行，可在审批时调整）
1. **IDCoin 发行功能全面下线**（/launch 部署、铸造、赎回 Redeem、退款、部署/铸造历史）——后端全是 orders.exchange 代理，合作方停运后已不可用；自建替代是大工程，不在本次范围。
2. **已发行的 IDCoin 保留市场交易**：上架/买卖/撤单/详情走自有后端（/common/*、/market/*、/inscribe/*），不受影响。
3. **getRawTx 替代**：后端新增 `GET /api/v1/common/tx/raw`（复用 own 服务 `GetTxRaw`），前端切换——同时修复当前上架报错。
4. **"ID Coins" 分类保留**，识别改为静态 tickId 列表（实施时从现存内部接口拉取全量约 32 个，后端仓库已有 `fixedTickIds` 硬编码先例 models/market_mrc20_info_model.go:160）。

## Phase 0：抓取静态列表数据
- curl 现存内部接口 `api.metaid.market/api-market/api/v1/id-coins/coins-list?cursor=0&size=100` 拉全量 idcoin tick+tickId，固化为后端常量。

## Phase 1：后端 metaid-market-service
1. **新增** `GET /api/v1/common/tx/raw?txId=`：common_controller 新 handler，调 `own_service.GetTxRaw`，返回 `{code,message,data:{rawTx}}`（与前端现有解构 `data.rawTx` 兼容）；http_controller.go 注册路由（无鉴权，公开数据）。
2. **静态 idcoin 列表**：common 包新增常量 `IdCoinsTickIds`（Phase 0 数据）。
3. **CheckIdCoins 改造**（common/common_service.go:337）：删除 `FetchOneIdCoinsInfo` 远程分支，`deployTime <= IdCoinsSignTimestamp` 的老币改为静态列表命中；保留新币本地签名校验分支。
4. **FetchMrc20IdCoinsTickAddressUtxos**（common_service/mrc20_service.go:1082）：`FetchIdCoinsTickIds` 调用改为静态列表。
5. **FetchMrc20TickAddressUtxos**（mrc20_service.go:788）等处的 tag 检测：`FetchOneIdCoinsInfo` 改为 `CheckIdCoins`/静态判断（tag 逻辑保留）。
6. **删除整个依赖包与代理层**：
   - `service/orders_exchange_service/`（整包，含 auto bridge 客户端）
   - `controller/id_coins_controller.go`、`controller/request/id_coins_request.go`、`controller/respond/id_coins_respond.go`
   - `service/mrc20_op_service/{deploy,mint,fetch,redeem,refund_invalid}_idcoins_service.go` 五个文件
   - `controller/http_controller.go` 删除 /id-coins/* 全部路由（L69-83）及 `/admin/auto-bridge/create`（L105）、`/admin/auto-bridge/info`（L106）
   - `service/admin_service.AddAutoCreateBridge`、`service/auto_service/auto_create_bridge_service.go`、`service/task/job_check_market_tick_bridge.go` + run_job.go 中注册
   - `models/id_coins_deploy_order_model.go`（仅死代码引用）、`models/market_auto_bridge_create_model.go`
   - `conf/init_conf.go`：删 `OrdersExchangeDomain/Key` 变量与 viper 读取（yaml 部署配置同步删 orders_exchange 节——写进交付说明给运维）
7. **验证**：`go build ./... && go vet ./...`。docs/docs.go（swagger 生成物）中的 id-coins 描述随文件删除自然失效，不重新生成 swag（备注）。

## Phase 2：前端 metaid-market-frontend
1. **api.ts**：
   - `getRawTx` 改为 `${getHost(network)}/api/v1/common/tx/raw`（getRawTx 是全站唯一直连 orders.exchange 的调用，改后 orders.ts:379/528 的 PIN 流程一并修复）
   - 删除注释掉的 orders.exchange broadcastBTCTx 死代码块（L526-541）
   - 删除 idcoin 专用 API：deployIdCoinPre/Commit、mintIdCoinPre/Commit、getIdCoinMintOrder、getIdCoinInscribeOrders、refundIdCoinPre/Commit、redeemPreview、redeemIdCoinPre/Commit、checkUserCanDeployIdCoin、getIdCoinList、getIdCoinInfo
2. **数据源替换**：
   - 市场 "ID Coins" tab（Mrc20List/IdCoins.tsx）：`getIdCoinList` → `getMrc20List`（/common/mrc20/tick/info-list，tag 由后端静态列表得出）过滤 `tag==='id-coins'`
   - 详情页 /idCoin/:tick 与 listIdCoin 头部信息：`getIdCoinInfo` → `getMrc20Info`（/common/mrc20/tick/info）；idcoin 专属字段（followersLimit、liquidityPerMint、ordersPool、ordersPrice）从 UI 移除
3. **删除页面/组件**：
   - /launch 整页（含 IdCoinDetail、ComfirmLaunch 组件）+ .umirc.ts 路由 + 导航入口
   - inscribe 页 ID-coin 铸造分支：Mrc20Form 的 idcoin 分支、IdCoinCard.tsx、ComfirmMintIdCoin；ID Coins 列表 "Mint" 按钮入口
   - mrc20History 的 "ID-Coins Deploy"/"ID-Coins Mint" 两个 tab（DeployIdCoin.tsx、MintIDCoins.tsx 含退款流程）
   - ConfirmRedeem 组件 + idCoin.tsx 的 Redeem 入口
   - `utils/idcoin.ts`（部署/铸造/退款/赎回 PSBT 构建器）、`components/LiqPerMintNotice`
4. **合作方文案/链接清理**：
   - `config/index.ts`：TESTNET/MAINNET_ORDERS_TRADE_URL、getOrdersTradeUrlByNet
   - IdCoins.tsx / Mrc20List/IdCoinCard.tsx 的 "Trade" 按钮 + showTradeNotice 弹窗
   - idCoin.tsx：orders logo（assets/image.svg import）、Trade 卡片/链接、DEX tooltip、ordersPool/ordersPrice 展示；idCoin.less 的 .orders 样式
   - locales/zh-CN.ts 的 Orders 相关键；删除 assets/image.svg
   - 保留：utils/orders.ts（通用 PSBT 逻辑，仅换 getRawTx 源）
5. **死代码**：sale/index.tsx、listIdCoin/index.tsx 未使用的 buildAskLimit 导入等；typings.d.ts 中 idcoin 专用类型字段清理。
6. **验证**：`pnpm install && pnpm tsc --noEmit`（或项目等效类型检查）+ `pnpm build`。

## Phase 3：端到端验证与交付说明
- 后端起本地服务（或部署后）curl 新 `/api/v1/common/tx/raw` 验证返回 rawTx hex。
- 前端上架流程走查：listMrc20Order 不再请求 www.orders.exchange。
- 交付说明（给运维）：部署 yaml 删除 orders_exchange.domain/key 配置；数据库表 tb_id_coins_deploy_order、tb_market_auto_bridge_create 保留历史数据不动；swagger docs 未重新生成。

## 明确不动的东西
- 自有市场订单体系（/market/*、models/orders、前端 "My Orders"/history 页）——"orders" 字样中属于自有功能的全部保留。
- man 服务、own 服务、gRPC 资产服务等其他下游。
- 已发行 IDCoin 的上架（/list/idCoins/:tick）、买卖、撤单、详情页——全部保留且经此修复后恢复正常。