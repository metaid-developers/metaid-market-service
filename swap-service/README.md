# MetaID.market Swap Service (`swap-service`)

非托管 MVCSwap AMM 交易构建、按池串行队列、0-conf 追踪与管理员池注册服务。

## 启动服务

```bash
export PATH="$HOME/.nvm/versions/node/v20.20.0/bin:$PATH"
# 启用受保护的管理员热管理接口（可选；未设置时 /internal/swap/admin/* 默认 503 禁用）
export SWAP_ADMIN_TOKEN="replace-with-strong-random-secret"
export SWAP_PORT=18080
npm start
```

> 合约产物会严格按 `contracts/swap/*` 隔离缓存在 `data/compiled-swap/`（不会被 `contracts/swapRange/*` 覆盖），首次预热后冷启动耗时 < 1 秒。

---

## 管理员添加 / 管理交易对

前端只展示 `pools.json` 中 `verified: true` 且 `swapCodeHash` 在 `whitelistSwapCodeHashes` 白名单内的交易对。  
`/internal/swap/admin/*` 接口**不会**经由 Go 公开代理 `/api/v1/swap/*` 暴露，仅限内网或管理员命令行使用。

### 1. 查看当前已上线交易对

```bash
npm run admin:pool -- list
# 或查询运行中的服务：
npm run admin:pool -- list --api http://127.0.0.1:18080 --admin-token "$SWAP_ADMIN_TOKEN"
```

### 2. 导入链上已有的 MVCSwap 白名单池子（自动解析与密码学校验）

只需提供链上 `poolTxid` 和 `lpGenesisTxid`，工具会自动：
- 校验 `swapCodeHash` 在 `whitelistSwapCodeHashes` 白名单内；
- 自动解析 `token2` 的 `tokenID / codeHash / symbol / name / decimals`；
- 自动推导 6 个操作合约的 `contractHashArrayHex` 并与链上 `contractHashRoot` 逐字节比对；
- 自动核对 `fetchTokenHash` 与 `lpTokenID` 绑定关系，并检查储备非零。

```bash
# 先干跑演练校验（不写入）
npm run admin:pool -- import --dry \
  --pool-id space-msp \
  --pair "SPACE / MSP" \
  --pool-txid <POOL_HEAD_TXID> \
  --lp-genesis-txid <LP_GENESIS_TXID> \
  --lp-genesis-vout 2

# 确认无误后热加载到运行中的 swap-service（自动持久化 pools.json，无需重启）
npm run admin:pool -- import \
  --pool-id space-msp \
  --pair "SPACE / MSP" \
  --pool-txid <POOL_HEAD_TXID> \
  --lp-genesis-txid <LP_GENESIS_TXID> \
  --lp-genesis-vout 2 \
  --api http://127.0.0.1:18080 \
  --admin-token "$SWAP_ADMIN_TOKEN"
```

### 3. 从 0 在 MVC 主网创建全新交易对 + 注入首笔初始流动性

自动完成 7 笔创世链路：`T4a 拆分资金 → T7 LP Genesis → T5 SwapMain Genesis → T6b Check 合约 → T6c 存入初始 FT（支持余额找零）→ T6a AddLiq Op → T6d 首笔 AddLiquidity`。  
每笔交易在广播前都会在本地完成全合约 `.verify()` 密码学校验。

```bash
# 第一步：默认 --dry 演练模式（读取真实钱包 UTXO，离线构建并校验全部 7 笔交易，不广播不花钱）
SWAP_ADMIN_WIF="<你的管理员WIF>" npm run admin:pool -- create --dry \
  --token-id <FT_TOKEN_ID_40HEX> \
  --token-codehash <FT_CODEHASH_40HEX> \
  --init-space 1000000 \
  --init-token 1000000000 \
  --pool-id space-newtok \
  --pair "SPACE / NEWTOK"

# 第二步：确认费用与开盘价格无误后，加 --live 真链部署并热上线到运行中的服务
SWAP_ADMIN_WIF="<你的管理员WIF>" npm run admin:pool -- create --live \
  --token-id <FT_TOKEN_ID_40HEX> \
  --token-codehash <FT_CODEHASH_40HEX> \
  --init-space 1000000 \
  --init-token 1000000000 \
  --pool-id space-newtok \
  --pair "SPACE / NEWTOK" \
  --api http://127.0.0.1:18080 \
  --admin-token "$SWAP_ADMIN_TOKEN"
```

> 安全提示：推荐使用 `SWAP_ADMIN_WIF` 环境变量或 `--key-file /secure/path/key.json` 传入私钥，切勿将私钥写入 git 仓库或日志。

### 4. 下架交易对

```bash
npm run admin:pool -- remove \
  --pool-id space-newtok \
  --api http://127.0.0.1:18080 \
  --admin-token "$SWAP_ADMIN_TOKEN"
```

---

## 受保护的管理员 HTTP 接口

所有接口需携带请求头 `X-Swap-Admin-Token: <SWAP_ADMIN_TOKEN>`（或 `Authorization: Bearer <token>`）：

| 方法与路径 | 说明 |
| --- | --- |
| `GET /internal/swap/admin/pools` | 返回白名单哈希与当前所有已注册池配置 |
| `POST /internal/swap/admin/pools/import` | 链上解析、校验并热导入已有白名单池（支持 `dryRun: true` / `overwrite: true`） |
| `POST /internal/swap/admin/pools/register` | 校验并热注册完整 `PoolRegistryEntry`（供 `create --live` 自动热推送） |
| `DELETE /internal/swap/admin/pools/:poolId` | 热下架指定交易对并原子更新 `pools.json` |
