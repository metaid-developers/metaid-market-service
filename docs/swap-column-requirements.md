# MetaID.market「Swap」栏目需求文档（含技术交接）

> 日期：2026-11-23　状态：待产品方评审
> 读者：接手开发的新 Agent / 开发者（假定读者对 MVC 合约没有任何前序上下文）
> 本文是交接的**唯一权威入口**。过程记录见 `mvcswap-core-mainnet/docs/MAINNET-VALIDATION-STATE.md`，两者冲突时以本文为准。

---

## 0. 如何使用本文档

1. 先读第 1–4 节，弄清目标、边界和"哪些已经做完、放在哪里"。
2. **第 7 节是这份文档最重要的部分**：每一条都是主网上真实踩过、花了大量时间才定位的问题。照做即可，不要重新推导，也不要"优化"掉。
3. 第 8 节列的是**还没有验证**的事。第 9 节的 Phase 0 要求先把其中几项补齐，再进入正式开发。
4. 文中对每条结论都标注了证据来源（交易哈希或脚本路径）。拿不准时回到证据本身核对。

---

## 1. 背景与目标

MVC 链上原先有一个基于 sCrypt 智能合约的 AMM 交易所 MVCSwap（mvcswap.com），已于 2026-10-01 关停，其后端从未开源。我们现在拿到了它的合约与 SDK 源码快照（mvcswap-core），并且**已在 MVC 主网上把它完整跑通**：从零部署代币与池子、添加流动性、完成一笔兑换，全部被主网节点接受。

本项目的目标是在 MetaID.market 中新增「Swap」栏目，把 MVCSwap 的核心能力恢复过来：用户连接钱包后，可以对自己钱包里的 MetaContract 资产做**开池、添加流动性、撤出流动性、兑换（Swap）**，操作体验与原 MVCSwap 类似。

一句话判断：**合约层面已经用主网真实交易证明可行。但有一个必须先排除的风险：整个验证过程没有经过钱包（见 8.1），而产品形态恰恰依赖钱包签名。** 除此之外，剩下的主要是工程实现，以及第 8 节列出的几项需要补做的验证。

---

## 2. 范围

### 2.1 本期要做

| 功能 | 说明 |
|---|---|
| 兑换 SPACE → Token | 用 SPACE（MVC 原生币）买入某 MetaContract FT |
| 兑换 Token → SPACE | 卖出某 MetaContract FT 换回 SPACE |
| 开池 | 为某个 MetaContract FT 创建 SPACE 交易对并注入第一笔流动性 |
| 添加流动性 | 向已有池子注入 SPACE 与 Token，获得 LP Token |
| 撤出流动性 | 归还 LP Token，取回 SPACE 与 Token |
| 池子列表与详情 | 储备、价格、费率、LP 信息 |
| 我的持仓 | 用户在各池的 LP 余额 |
| 交易状态与记录 | 广播、确认、失败提示；历史记录 |

交易对类型本期只做 **SPACE ↔ Token**（对应 mvcswap-core 的 `contracts/swap/` 合约族）。

### 2.2 本期不做

- 流动性挖矿、质押、投票上币、Boost（原 MVCSwap 的 farm / stake / vote / boost）
- Token ↔ Token 交易对（`contracts/tokenSwap/`）
- 区间流动性池（`contracts/swapRange/`）
- 多跳路由
- K 线与统计
- MRC20 ↔ MetaContract 跨链桥，以及 MRC20 详情页的"桥接到 MVC"入口（另行立项，本文档不涉及）

### 2.3 与现有 MRC20 市场的关系

Swap 栏目只处理 **MVC 链上的 MetaContract FT**，与 BTC 链上的 MRC20 市场是两条独立的链。用户钱包里要先有 MetaContract FT 才能使用 Swap。两者在界面上如何关联（例如在 MRC20 详情页提示"该资产在 Swap 栏目可交易"）留到后续版本。

---

## 3. 决策事项

### 3.1 钱包（已确认：Metalet）

- 使用 **Metalet** 钱包（浏览器注入对象 `window.metaidwallet`），与现有 MetaID.market 和原 MVCSwap 一致。同一把密钥同时管理 BTC 与 MVC 地址。
- 前端依赖 `@metaid/metaid@1.5.9` 已经导出了 MVC 侧的连接器（`mvcConnect`、`MetaletWalletForMvc`），只是目前没用上。
- 需求口述中的"MetaMask"是语音识别错误。MVC 不是 EVM 链，本项目不涉及 MetaMask。
- 选型已定不等于能力已验证：Metalet 能否完成本项目所需的签名，见 8.1。

### 待确认事项

以下几项不该由开发方自行决定。**建议在开工前给出答案**，至少第 3.2 必须先定。

### 3.2 签名与信任模型

- **建议采用非托管模式**：后端负责构造整笔交易（包括所有合约输入的解锁脚本，这些不需要用户私钥），前端让用户钱包对其中属于用户的输入签名，然后广播。资金全程不经过平台地址。
- 原 MVCSwap 采用的是另一种模式：用户先把资产签名转给服务端控制的地址，再由服务端代为执行合约。这种模式平台需要托管资金，不建议沿用。
- 采用建议模式的前提是钱包能对"包含合约输入的整笔交易"签名。**这一点还没有验证**，是当前最大的未知项，见 8.1。

### 3.3 谁可以开池，手续费归谁

- 合约层面**任何人**都可以为任何 MetaContract FT 开池，不需要许可。
- 每个池子在创建时写入三个手续费参数和一个管理员地址。费率按成交额的万分比计：`feeRate` 是兑换费率（25 即 0.25%）；`projFeeRate` 是平台分成（5 即 0.05%），单笔分成低于 `minProjFee` 时不收；`projFeeAddress` 是收款地址。**管理员之后可以在链上修改费率和收款地址**。验证池用的是 25 / 5 / 测试钱包地址 / 1000 sats。
- 需要决定：平台是否只展示自己部署（或审核过）的池子；平台池的费率与收款地址；管理员地址由谁保管。
- 安全提示：如果栏目允许展示任意的池子，会有人部署高费率或钓鱼池。建议**池子列表只展示经过平台后端校验的池**（后端按合约代码哈希白名单过滤，见 7.3），不接受用户自行输入池子标识来交易。

### 3.4 首期支持哪些 MetaContract 资产

需要一份清单。每个资产上线前要做一项校验，见 8.4。

---

## 4. 现状盘点：已经完成了什么，在哪里

### 4.1 已在主网验证通过的交易（全部被节点接受）

| 步骤 | 对应脚本 | 交易哈希 |
|---|---|---|
| 代币创世与发行 | `scripts/phase1-genesis.ts` | `498a5b0f…344b`、`876c698c…b3cb` |
| 资金拆分（产生池子身份锚点） | `scripts/phase2-swap-genesis.ts` | `b2d6edec…2e27` |
| LP 代币创世合约部署 | 同上 | `6b8626bf…5cf2` |
| 池子创世合约部署 | 同上 | `69340f1c…3be9` |
| 创建校验合约实例 | `scripts/phase3-addliq.ts` | `5e56718b…f759` |
| 代币转入"添加流动性"合约实例 | 同上 | `279aff17…6a59` |
| 创建"添加流动性"操作合约实例 | 同上 | `85b9e5a5…548c` |
| **首笔添加流动性** | `scripts/phase3d-resume.ts` | `455b925f…e9db` |
| 创建兑换所需的合约实例 | `scripts/phase3b-swap.ts` | `81971575…6e69` |
| **首笔兑换（10,000 sats → 153,343,581 MVST）** | 同上 | `111e063a…f128` |

当前这个池子仍然有效，可以直接拿来继续测试（状态见附录 A）。MVST 是我们自建的无价值测试代币。

### 4.2 各仓库状态

| 仓库 / 目录 | 分支 | 状态 |
|---|---|---|
| `mvcswap-core` 的 worktree：`/Users/tusm/Documents/MetaID_Projects/mvcswap-core-mainnet` | `feat/mainnet-validation` | 所有验证脚本与记录都在这里。**未合并回 main，未推送** |
| `mvcswap-core` 主目录：`/Users/tusm/Documents/MetaID_Projects/mvcswap-core` | `main` | 原始快照，未改动 |
| higun 的 worktree：`/Users/tusm/Documents/MetaID_Projects/higun-worktrees/codex-swap-data-layer` | `codex/swap-data-layer` | swap 数据层已实现并通过评审。**未合并、未部署**。分支末尾 3 个提交是 metaid.market 侧补充的对照测试与文档 |
| `/Users/tusm/Documents/MetaID_Projects/metaid-market-frontend` | main | 尚未开始 |
| `/Users/tusm/Documents/MetaID_Projects/metaid-market-service` | main | 尚未开始 |
| `/Users/tusm/Documents/MetaID_Projects/mvcswap` | — | 原 MVCSwap 前端的 2024-01 旧版，仅作交互与算法参考 |

`mvcswap-core-mainnet/scripts/` 下的脚本是**验证脚本，不是生产代码**：其中 `phase3-addliq.ts` 含大量调试残留（逐条复算、追踪钩子），`probe-*.ts` 是排查过程的一次性实验。请把它们当作"如何正确调用合约的可运行范例"来读，不要直接当模块复用。

---

## 5. 目标架构

```
浏览器（metaid-market-frontend，新增 Swap 栏目）
   │  Metalet 钱包：对用户输入签名
   ▼
metaid-market-service（现有 Go/Gin 服务）
   │  登录态校验、限流、记录、转发
   ▼
swap-service（新增，Node.js / TypeScript）
   │  报价、构造交易、池状态缓存、同一池子的交易排队
   ▼
higun（现有 Go 索引器，待合并 swap 数据层）
   │  取原始交易、广播、按白名单查询池子 UTXO
   ▼
MVC 主网
```

设计要点：

- **swap-service 必须用 TypeScript**。合约的构造与解锁逻辑只存在于 `mvcswap-core` 的 TS 库（`tslib/`）和 `mvc-scrypt` SDK 中，Go 侧没有等价实现。生态里已有同类先例：`metacontract-peripheral-service` 就是因为"合约交易构建无法在 Go 侧实现"而独立成服务的。
- 现有 Go 服务不需要理解合约，只做代理与治理。
- 建池（开池）是一次性的管理操作，可以先做成运维脚本（已有可运行范例 `phase2-swap-genesis.ts`），不必在第一版就做成用户功能。
- higun 的 `/swap/utxos` 接口负责回答"某个池子当前的 UTXO 在哪"。swap-service 拿到 `scriptHex` 后用 `tslib/tokenSwapProto.ts` 的 `getSwapInfo` 解析储备（已验证可直接读主网池子）。

---

## 6. 用户流程与链上交易序列

MVC 的合约是 UTXO 模型：每个操作要花一到多笔交易，后一笔花前一笔的输出。**主网已验证可以直接花费未确认的输出**（0 确认链式广播），所以用户在界面上看到的是"一次操作"，链上是连续广播的几笔交易。

通用模式：每个操作合约在使用前，必须先作为一笔交易的输出被"创建"出来（下称**实例**）。实例里编码了发起人地址，是按用户区分的。创建实例的交易由用户钱包发起即可，不需要平台经手资金。

| 操作 | 交易序列 | 主网验证情况 |
|---|---|---|
| 添加流动性 | ① 创建校验实例 → ② 用户把 Token 转入"添加流动性"合约实例的地址 → ③ 创建"添加流动性"操作实例 → ④ 添加流动性（同时完成 LP 铸造） | 已验证（首笔，池子从零储备起步） |
| 兑换 SPACE → Token | ① 创建操作实例 → ② 兑换 | 已验证（一笔） |
| 兑换 Token → SPACE | 大致需要：用户把 Token 转入"卖出"合约实例的地址、创建实例、兑换。具体步骤与顺序未验证，以离线测试和合约代码为准 | **未验证** |
| 撤出流动性 | 大致需要：用户把 LP Token 转入"撤出"合约实例的地址、创建实例、撤出。具体步骤与顺序未验证 | **未验证** |
| 开池 | ① 资金拆分 → ② 部署 LP 创世合约 → ③ 部署池子创世合约 → ④ 按上面的流程做首笔添加流动性 | 已验证（一次） |

参照规格的位置（都在 `mvcswap-core-mainnet/tests/ts/`）：
- `swapMain.scrypttest.ts`：交易如何拼装——`addLiq`（约 507 行）、`removeLiquidity`（约 740 行）、`swapToken1ToToken2`（约 1007 行）、`swapToken2ToToken1`（约 1160 行）；每个操作合约的解锁参数——同文件里的 `unlockMvcToToken`、`unlockTokenToMvc`、`unlockAddLiq`、`unlockRemoveLiq`。
- `swapTestHelper.ts`：`unlockSwap`（池子）、`unlockLpGenesis`、`unlockFetchToken`。

**注意**：离线测试里的交易是用占位数据拼出来的，直接照搬会在主网失败。主网可用的写法以 `scripts/phase3-addliq.ts`、`phase3d-resume.ts`、`phase3b-swap.ts` 为准，区别正是第 7 节的内容。

---

## 7. 已攻克的技术难点（必须遵守）

每一条都给出现象、原因和正确做法。**这些是主网验证阶段耗时最多的部分，请逐条对照实现。**

### 7.1 含 OP_CODESEPARATOR 的合约，预镜像必须用"分隔符之后的脚本"计算

- **现象**：解锁校验报 `SCRIPT_ERR_NULLFAIL, fails at OP_CHECKSIG`。
- **原因**：这类合约的编译产物里有一个 `OP_CODESEPARATOR`，最终的签名检查只覆盖它**之后**的那段脚本。用整段脚本算预镜像，对不上。
- **做法**：`getPreimage(tx, contract.lockingScript.subScript(0), amount, inputIndex, sigType)`。`subScript(0)` 的含义是"第 0 个分隔符之后的脚本"。
- **适用范围**：可以**对所有 swap 族合约统一加 `.subScript(0)`**。对不含分隔符的脚本它原样返回整段脚本，没有副作用（已读 SDK 源码确认）。
- 已验证的具体用法：操作合约（`swapMvcToToken`、`swapAddLiquidity`）、`swapMain`、`swapFetchToken`、两种校验合约，都用了 `subScript(0)`；`Token.unlock` 与 `swapLpGenesis` 用的是整段脚本，也通过了。

### 7.2 合约实例必须带上真实的 data part

- **现象**：签名校验失败；或者实例脚本长度与链上脚本差几字节。
- **原因**：`new SomeContract(...)` 只是换了构造参数的模板，脚本还处于占位形态，和链上真实脚本不一致。解释器按实例脚本算出的内容自然对不上。
- **做法**：实例化后调用 `setDataPart(...)`，并且**在构造交易前断言实例脚本与链上脚本逐字节相等**：
  `inst.lockingScript.toBuffer().equals(chainScriptBuf)`。不等就直接报错，不要继续。
- data part 怎么来的：
  - Token 与 LP 创世合约实例：取链上脚本末尾 170 字节（`script.subarray(script.length - 170)`）。
  - 池子实例（`swapMain`）：按业务字段重建（字段顺序见 `phase3b-swap.ts` 第 139 行附近）。直接对链上脚本切片的写法试过 193 和 213 两个长度都没成功，原因没有查明，所以别用切片。
- 这条对 Token 实例尤其容易漏：Token 合约没有分隔符，症状只会表现为签名错误，而不是长度错误。

### 7.3 代码哈希（白名单）的正确算法

- **现象**：`Token.unlock` 报 `SCRIPT_ERR_VERIFY`，其它检查全部通过。
- **原因**：Token 合约里烧着一组"允许哪些校验合约来花我"的代码哈希。我们用两种自创的"从尾部剥离"算法算出的值都是错的。
- **正确算法**：`hash160(裸实例脚本 + 0x6a)`。"裸实例"指只填了构造参数、没有 `setDataPart` 的实例，`0x6a` 是一个字节的 `OP_RETURN`。这与测试代码 `token.scrypttest.ts` 的 `initContractHash()` 完全一致，也与"对带规范 data part 的实例调用 `getScriptCodeHash`"结果相同（两者相等这一点已单独验证）。
- 用这套算法算出的两个值，和生产环境里的 MetaContract 资产完全一致：
  - `TokenTransferCheck`：`7145466ef00ad6c9d72c665ca553cd8e8031102f`
  - `TokenUnlockContractCheck`：`1849eb9ae201d34383c415448da74b8419f13f0c`
- **做法**：在代码里运行时计算，不要硬编码。验证脚本里仍有硬编码的旧值残留（如 `93ba0e6a…`、`cc537f3b…`、`24e2bbe7…`），**那些都是错的**。
- 池子代码哈希同理：`hash160(裸 SwapMain 实例脚本 + 0x6a)`，值为 `84c4452c13885d1538977e500e47f767910c668c`；已验证它在创世、添加流动性、兑换三种状态下都不变。higun 的白名单索引用的就是它。

### 7.4 转账和解锁要用两种不同的校验合约

- Token 被"普通转账"花费时（用户签名授权，操作码 1），校验合约必须是 `TokenTransferCheck`。
- Token 被"合约"花费时（无签名，由操作合约的规则授权，操作码 2），校验合约必须是 `TokenUnlockContractCheck`。
- 两者的数据格式不兼容：后者多一个"输入索引数组"前缀。给错的话，轻则校验合约自己解不开，重则报 `INVALID_SPLIT_RANGE`。
- 兑换与添加流动性中同时用到两种。具体的数据布局与解锁参数分别见 `phase3-addliq.ts` 里的 `buildTransferCheckInstance` / `buildCheckInstance`，以及 `token.scrypttest.ts` 的 `unlockTransferCheck`（约 170 行）。

### 7.5 Token 要先转入操作合约实例的地址，才能被合约花费

操作合约（添加流动性、卖出）要动用用户的 Token 时，这些 Token 的持有人必须是**该操作实例脚本的哈希地址**，即 `ripemd160(sha256(实例脚本))`。所以每个操作前都有一笔用户签名转账（操作码 1），把 Token 转到这个地址。转入之后由操作码 2 解锁。地址算错的症状是 `EQUALVERIFY`。

### 7.6 操作码 2 的几个参数容易填错

| 参数 | 正确值 |
|---|---|
| `senderPubKey` / `senderSig` | 填 33 字节、72 字节的**全零**数据（不能传空串，SDK 会报类型错误） |
| `contractInputIndex` / `contractTxProof` | 指向**当前持有这枚 Token 的那个合约实例**在本交易中的输入位，及其创建交易的输出证明。池子里的 Token 由 `swapFetchToken` 实例持有；用户转入的 Token 由对应的操作实例持有 |
| `prevTokenAddress` / `prevTokenAmount` | 这枚 Token 的**上一位持有人**的地址与当时数量（不是本次转出后的） |
| `prevTokenInputIndex` | 创建这枚 Token 的那笔交易里，花费上一枚 Token 的输入位。例如添加流动性产生的池 Token，这个值是 3 |

### 7.7 池子的创世仪式

- 池子的身份来自**创建它的那笔交易自己所花费的某个输入**（记作 F5），而不是池子自己的交易哈希。所以流程是：先发一笔"资金拆分"交易得到一个专门的输出作为 F5，再据此推出 `swapContractID = hash160(F5)`，然后所有操作合约的代码哈希、`contractHashRoot`、`lpTokenID`、`fetchTokenHash` 都能在广播池子之前算出来。
- LP 创世合约必须先于池子部署，因为池子数据里要写入 `lpTokenID`。
- LP 创世合约实例随每次添加流动性链式续约，首次添加流动性时完成 LP 铸造。
- 完整可运行范例：`scripts/phase2-swap-genesis.ts`。

### 7.8 `tokenScriptCodeHash` 必须按该 Token 的真实链上脚本计算

白名单是 Token 合约代码段的一部分，所以不同 Token 的代码哈希不同（WBTC、MetaCoin、MVST 三者各不相同）。不要用一个"标准 Token 实例"去算。做法：从链上取该 Token 任意一个 UTXO 的脚本，调用 `TokenProto.getScriptCodeHash`。

### 7.9 输出顺序是合约规则的一部分

操作合约会逐字节重建本交易的全部输出并比对哈希。顺序错一位就失败。
- 兑换（SPACE → Token）：新池子、用户得到的 Token、池 Token 余额、SPACE 找零、平台手续费。手续费一笔不足最低值时没有手续费输出，其余四个输出的顺序不变。
- 添加流动性：新池子、LP Token、LP 创世合约续约、池 Token 余额、找零。
- `TokenUnlockContractCheck` 需要你把"非 Token 输出"按原始格式加上 4 字节长度前缀传进去，并给出 Token 输出所在的输出序号。`TokenTransferCheck` 的传参方式不同，它直接接收 Token 输出金额、找零金额、找零地址。

### 7.10 池子储备要从脚本数据段读，不能用 UTXO 的金额

池子 UTXO 的金额比 SPACE 储备多一个固定底座（当前池子是 100,100 sats）。读储备用 `TokenSwapProto.getSwapInfo(script, len)`。

### 7.11 广播与装配

- 解锁脚本不会自动写入交易：`verify()` 只做校验。校验通过后必须 `tx.inputs[i].setScript(result.unlockingScript)`。
- 合约交易的序列化必须用 `tx.uncheckedSerialize()`。`serialize()` 会做客户端检查，不认合约脚本而报错。

### 7.12 解析编译产物时的陷阱

编译器 CLI 输出的十六进制模板里含有 `<变量名>` 之类的占位符。`Buffer.from(hex, 'hex')` 遇到非法字符会**静默截断**，只得到开头一小段，看起来就像"编译器只生成了骨架"。排查时曾因此误判了好几轮。解析任何来自文件或 CLI 的十六进制前，先确认长度符合预期。

### 7.13 校验失败时怎么定位

`verify()` 的报错只给出操作码级别的原因（如 `OP_VERIFY`），不告诉你是哪一条检查失败了。有效办法：给 SDK 自带的脚本解释器打点，在失败瞬间打印栈顶内容。可直接复用 `scripts/trace-hook.ts`（用法见 `phase3-addliq.ts`）。白名单哈希的错误就是靠它定位的。
`scripts/tracer-lib.ts` 是手写的栈机模拟，已知有语义偏差，不要依赖。

### 7.14 广播时的运维经验

- 一笔交易可能花光钱包的 UTXO，后续交易要改为花费前一笔的找零输出。
- 中途放弃的运行留下的交易仍在内存池里，再次运行时会"抢走"上一轮的找零，导致 `txn-mempool-conflict`。写广播逻辑时要支持重试与幂等（交易已被节点认识时按成功处理）。
- 从 metalet 的接口读取未确认交易偶尔会瞬时失败或限流，必须带重试。曾因为读取失败后用了备用计算路径，把错误的值写进了链上，白部署了两个池子。**原则：读不到就中止，不要用推算值顶替。**

---

## 8. 尚未验证的事项

下面这些**没有**在主网上跑过。请不要把它们当成已知可行。

### 8.1 钱包签名（风险最高，先做）

钱包已确定为 Metalet（见 3.1），但整个主网验证阶段，所有签名都是用本地私钥文件直接完成的，**没有经过任何钱包**。需要验证 Metalet 能否完成两件事：

1. 对一笔包含合约输入的交易，只对其中属于用户的 P2PKH 输入签名（并保持其它输入的解锁脚本原样不动）。
2. 对 Token 转账（操作码 1）生成用户签名 `senderSig`。

线索：原 MVCSwap 前端调用过 `window.metaidwallet.signTransaction({ transaction })` 和 `window.metaidwallet.transfer({ broadcast: false })`（见 `mvcswap/src/lib/metaWallet.js` 第 84、131 行），说明钱包有相关接口，但它接收的格式、返回的内容，以及能否满足上面两点，都没有核实。MetaID.market 现有代码里没有对 MVC 交易签名的先例。

如果钱包做不到第 1 点，3.2 建议的架构就不成立，需要重新设计（退回托管模式，或请钱包增加接口）。所以这一项必须最先验证，它的结果决定后面怎么做。

### 8.2 每种操作只在主网跑过"第一笔"

| 未验证的场景 | 为什么有风险 |
|---|---|
| 第二笔及以后的兑换 | 第一笔兑换时池子刚完成添加流动性，合约走的是"创世后第一跳"的宽松分支。之后的兑换会走完整的链上回溯校验，需要额外传入上一状态的 `prevCustomData`。只有离线测试覆盖 |
| Token → SPACE 方向 | 完全没在主网跑过 |
| 第二笔及以后的添加流动性 | 第一笔是从零储备起步，不检查比例；之后会检查注入比例并按比例铸造 LP |
| 撤出流动性 | 完全没在主网跑过 |
| 平台手续费大于零的兑换 | 当前池子的最低手续费是 1000 sats，测试金额小，实际手续费为 0，手续费输出这条路径没走到 |
| 把多个合约实例合并到一笔交易里创建 | 验证时是分几笔创建的。合并可以省手续费和交易数，但没试过 |

一个有利的经验：验证期间广播的合约交易，**离线校验通过的，在主网上全部被接受**（样本是我们广播的那几笔）。所以这些场景可以先在现有池子上离线校验，通过后再花小额上链确认。

### 8.3 higun 的 swap 数据层还没有合并和部署

代码在分支 `codex/swap-data-layer` 上，评审已通过（含用我们真实池子脚本做的对照测试），但没有合并进 main，也没有部署。提供的接口：`GET /mvc/tx/:txid/raw`、`POST /mvc/broadcast`、`GET /swap/utxos`。上线前需要与 higun 团队确认合并与部署时间，以及生产环境的访问地址和端口。

### 8.4 用生产资产端到端跑一遍

已确认生产环境的 WBTC、MetaCoin 的 Token 合约里烧着与我们相同的校验合约哈希（见 7.3），所以**理论上**同一套 swap 合约可以直接为它们开池。但这个结论只停留在白名单层面，没有真的用生产资产做过兑换。每个要上线的资产，上线前都要校验它的白名单，并实际跑一笔小额兑换。

### 8.5 纯工程问题（验证阶段没有涉及）

- **手续费估算**：验证时用的是写死的金额。生产要按交易体积估算；合约交易体积大（数 KB 到数十 KB），MVC 对交易体积和手续费的限制没有研究过。
- **同一池子的并发**：池子只有一个 UTXO，两笔交易同时花它会互相冲突。需要 swap-service 对同一池子的交易排队，冲突后基于最新状态重新构造。从原 MVCSwap 的接口设计看，它的后端（未开源）承担的就是这个职责。
- **链重组与确认**：0 确认链式交易在重组时如何处理，没有考虑。
- **池子发现**：靠 higun 的白名单索引。higun 的白名单在启动时加载，不回溯历史区块；新增白名单后需要从池子创世所在区块开始重建索引，或做全量重建。
- **服务安全**：swap-service 是内部服务，只应接受现有 Go 服务的调用；需要限流与监控。
- **私钥**：swap-service 不需要持有任何用户或平台的私钥。

---

## 9. 分期计划与验收标准

以下只给顺序，不给工期（工作量取决于 8.1 的结果）。唯一的粗略参考：Phase 0 是数天量级的工作。

### Phase 0：补做验证（必须先完成）

1. 验证 Metalet 的签名能力（8.1）。产出：一份明确的结论和可运行的签名示例代码。
2. 在现有的真实池子上，用脚本补跑主网：第二笔兑换、Token → SPACE、第二笔添加流动性、撤出流动性、一笔手续费大于零的兑换。每一步先离线校验，再广播。产出：更新后的范例脚本和交易哈希。
3. 验证 Metalet 签出的交易能被节点接受（把第 1 项和第 2 项合起来：至少完成一笔由钱包签名的兑换）。

验收：以上各项都有主网交易哈希作为证据；做不到的写明原因和替代方案。

### Phase 1：swap-service 核心（只支持已有池子）

- 从链上读取并缓存池子状态；报价（直接调用 `tslib/tokenSwapProto.ts` 的 `swapToken1ToToken2` 等函数，保证与合约一致；不要用旧前端的报价代码，它缺少平台手续费的逻辑）。
- 构造兑换交易（两个方向）；同一池子排队；广播与状态跟踪。
- 内部接口：报价、构造交易、提交已签名交易、查询交易状态。

验收：
- 报价函数的回归值：以储备 120,100 / 2,000,000,000、输入 10,000 sats 调用，输出必须是 153,343,581、平台手续费 0，与链上已成交的那笔一致。
- 两个方向的兑换各在主网成功至少 3 笔连续交易。
- 同一池子并发提交 2 笔时，两笔最终都成功，且没有资金丢失。

### Phase 2：添加与撤出流动性

验收：
- 第二笔添加流动性按比例铸造 LP，结果与 `countLpAddAmount` 的计算一致。
- 撤出流动性取回的资产与合约公式一致，LP 被销毁。
- 以上各在主网成功至少 2 笔。

### Phase 3：前端 Swap 栏目

- 导航新增「Swap」（位置：`src/layouts/components/Nav.tsx` 的 `items` 数组）；新增路由与页面目录。
- 接入 MVC 侧钱包连接；展示 SPACE 余额（钱包已有 `getMvcBalance` 接口）与 MetaContract FT 余额。
- 兑换页：选择资产、输入数量、显示可得数量与价格影响、滑点保护、确认与状态反馈。
- 流动性页：池子列表、添加、撤出、我的持仓。
- 只展示后端返回的、经过校验的池子（见 3.3）。

验收：用 Metalet 钱包在主网上以小额资金，通过页面完整走通兑换、添加、撤出各一次；每一步都有明确的进行中、成功、失败状态和交易哈希链接。

### Phase 4：higun 联调与上线

- higun 合并、部署，配置白名单 `swap_codehashes: [84c4452c13885d1538977e500e47f767910c668c]`、`swap_code_tail_len: 207`，确认 `/swap/utxos` 能列出测试池子的当前 UTXO（该池子在 Phase 0 之后还会被继续使用，以当时的最新状态为准；写本文时是 `111e063a…:0`）。
- swap-service 的池子发现切换到 higun；Go 侧加上代理、限流、监控。
- 开池工具（可先用脚本），按 3.3 的决定部署平台池。

---

## 10. 环境与操作纪律

### 10.1 运行环境

- **Node.js 20.20**：`export PATH="$HOME/.nvm/versions/node/v20.20.0/bin:$PATH"`。验证全程使用这个版本。仓库 README 记载 Node 26 下测试依赖链会报错（这一点我们没有亲自复现），建议直接沿用 20。
- 编译器：需要的版本已装在 `node_modules/mvc-scrypt/compiler/scryptc/mac/`。1.8.5 到 1.22.0 各版本编译出的产物长度一致，验证中没有遇到任何与编译器版本有关的问题，不需要在这方面花时间。
- 运行验证脚本：`npx ts-node --transpile-only scripts/<脚本>.ts`。不带 `--live` 只做离线校验，带了才广播。
- 合约描述文件：先运行一次 `node scripts/compile-all.js` 生成 `out/`。

### 10.2 没有测试网，所有链上操作都是真钱

MVC 已经没有测试网。主网验证全程用的小额真钱。请遵守：

- 任何交易先离线校验（`verify()`），全部通过再广播。
- 广播前做 7.2 的逐字节相等断言。
- 一次只广播一个步骤并确认节点接受，再进行下一步。不要写"一口气广播全部"的循环去跑没验证过的流程。
- 参考量级：整个验证阶段钱包从 1 SPACE 降到约 0.96 SPACE，共支出约 0.035 SPACE，其中包含手续费、锁在作废合约里取不回的资金，以及仍可从当前池子撤回的流动性。每次失误都是真实损失。

### 10.3 测试钱包

- 地址 `18HzEZDJ9wPxueKDCu1jvg1T5WtadHvNLd`，私钥文件 `mvcswap-core-mainnet/scripts/.keys/main.json`（已加入 `.gitignore`）。
- 余额 96,460,932 sats（约 0.96 SPACE），持有 LP 120,100 和 153,343,581 MVST（约 1.53 亿）。
- **这是测试资金，不能当作任何生产用途的密钥；任何时候都不要提交或外传。**

### 10.4 访问链上数据

- 验证脚本默认走 metalet 的公开接口 `https://www.metalet.space/wallet-api/v4/mvc`（本机可达，但有偶发限流）。用法见 `scripts/mainnet-lib.ts`。
- 生产的 `open-api-mvc` 由我们自己控制，但本开发机访问超时。切换方式见 `mainnet-lib.ts` 顶部的 `API_PROVIDER`。

### 10.5 关于临时目录

调研阶段克隆到 `/tmp` 下的仓库（如 `web-interface`、`swap-api`）已经被系统清理；`/tmp/mvclabs-research` 下的也随时可能消失。需要时请重新克隆，地址见第 11 节。

---

## 11. 资料索引

**本项目产出**
- 验证脚本与状态：`/Users/tusm/Documents/MetaID_Projects/mvcswap-core-mainnet/`（分支 `feat/mainnet-validation`）
  - `scripts/mainnet-lib.ts`：链上读写、钱包、交易辅助
  - `scripts/phase1-genesis.ts`、`phase2-swap-genesis.ts`、`phase3-addliq.ts`、`phase3d-resume.ts`、`phase3b-swap.ts`：各步骤的可运行范例
  - `scripts/trace-hook.ts`：解释器打点工具
  - `scripts/mainnet-state.json`：当前这组链上资产的完整参数（未纳入版本库，只存在于本机）
  - `docs/MAINNET-VALIDATION-STATE.md`：过程记录
- higun swap 数据层：`/Users/tusm/Documents/MetaID_Projects/higun-worktrees/codex-swap-data-layer/`，接口文档 `docs/swap-api.md`
- 需求与评审记录：`/Users/tusm/Documents/MetaID_Projects/higun/docs/swap-pool-data-requirements.md`

**合约与 SDK**
- `mvcswap-core-mainnet/contracts/swap/`：本期使用的合约（`swapMain`、`swapMvcToToken`、`swapTokenToMvc`、`swapAddLiquidity`、`swapRemoveLiquidity`、`swapLpGenesis`、`swapFetchToken`）
- `mvcswap-core-mainnet/contracts/token/`：Token 与两种校验合约
- `mvcswap-core-mainnet/tslib/`：`tokenSwapProto.ts`（池子数据解析、兑换公式）、`tokenProto.ts`、`common.ts`（证明构造、交易辅助）
- SDK 源码：https://github.com/mvc-labs/mvc-scryptlib
- Token 合约生产源码（与快照一致）：https://github.com/mvc-labs/token-core
- 生产 FT SDK：https://github.com/mvc-labs/meta-contract （桥后端 `octopus-bridge/backend-bridge` 中有它的服务端用法）
- 服务骨架参考：https://github.com/several-magicians/metacontract-peripheral-service

**原 MVCSwap 前端（仅参考）**
- 本地旧版：`/Users/tusm/Documents/MetaID_Projects/mvcswap`（`src/pages/swap/index.js` 交互、`src/lib/metaWallet.js` 钱包适配、`src/common/pairUtils.js` 报价）
- 较新版：https://github.com/mvc-swap/web-interface （2026-08）
- 原后端接口协议文档：https://github.com/mvc-swap/swap-api （含 `reqswapargs` 等接口的定义，可作为内部接口设计的参考；注意其原实现的信任模型见 3.2）

**MetaID.market 现有代码**
- 前端：`/Users/tusm/Documents/MetaID_Projects/metaid-market-frontend`（UmiJS 4、Ant Design 5；依赖 `@metaid/metaid@1.5.9`、`meta-contract@0.4.10`）
- 后端：`/Users/tusm/Documents/MetaID_Projects/metaid-market-service`（Go、Gin）

**节点源码**（需要确认脚本语义时查阅）
- `/Users/tusm/Documents/MetaID_Projects/microvisionchain/src/script/`（`interpreter.cpp`、`opcodes.h`）

**真实交易样本**
- 我们自己的：见 4.1
- 生产环境的真实兑换：`f439f627a0982e55cde74aba0fcb4d256ffa9a0d5a96bbe052dd65ae3393b2b4`（WBTC 交易对；注意它的解锁参数格式与合约快照不是同一代，不能直接照搬）
- 生产环境的真实 Token 转账：`ea3c2283b2c8d35cd4ec9f3fdca3d816c0cd8e40a424507f8e333db4cf9dfe3d`

---

## 附录 A：当前真实池子（可直接用于测试）

| 项目 | 值 |
|---|---|
| 池子当前 UTXO | `111e063ac20e59f2acfbe0d4c4659ca4025c74d6e798ae4719a362e57a97f128:0` |
| 池子 UTXO 金额 | 230,200 sats |
| 储备 | 130,100 sats / 1,846,656,419 MVST |
| LP 总量 | 120,100 |
| 池子代码哈希 | `84c4452c13885d1538977e500e47f767910c668c` |
| `swapContractID` | `5b041b1dae4a49f2cb7281e085f996533285738f` |
| 测试代币 | MVST，`tokenID` `5ec98a884f16931190f2fad564da7ac0a4a18436`，`tokenScriptCodeHash` `f725449d154e41a996412433cc41148e4355c7cc`，8 位小数 |
| 费率参数 | `feeRate` 25，`projFeeRate` 5，`minProjFee` 1000 sats |
| 已成交的兑换（可作报价的回归值） | 输入 10,000 sats，当时储备 120,100 / 2,000,000,000，输出 153,343,581，平台手续费 0 |

其余参数（`contractHashRoot`、各操作合约代码哈希、`lpTokenID` 等）见 `scripts/mainnet-state.json`。
