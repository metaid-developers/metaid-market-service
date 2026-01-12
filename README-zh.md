# metaid-market-service

中文 | [English](README.md)

MetaID.Market 平台后端服务。

## 功能特性

- 市场订单管理（NFT/代币订单）
- MRC20 代币操作（部署、铸造、转账）
- ID Coins 管理
- MetaName 注册
- UTXO 管理
- 交易广播

## 环境要求

- Go 1.20+
- MySQL
- Redis

## 快速开始

1. 克隆仓库
2. 复制 `conf/conf_example.yaml` 到 `conf/conf_test.yaml` 并配置
3. 运行服务（指定环境）：
   ```bash
   go run main.go -env=testnet  # 或 mainnet, loc
   ```

## 配置说明

编辑 `conf/conf_*.yaml` 文件进行配置：
- 数据库连接
- Redis 连接
- 外部服务端点
- 平台手续费

## API 文档

启动服务后，访问 `/swagger/index.html` 查看 Swagger API 文档。

## 许可证

查看 LICENSE 文件。
