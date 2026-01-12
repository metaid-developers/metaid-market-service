# metaid-market-service

[中文](README-zh.md) | English

Backend service for MetaID.Market platform.

## Features

- Market order management (NFT/Token orders)
- MRC20 token operations (deploy, mint, transfer)
- ID Coins management
- MetaName registration
- UTXO management
- Transaction broadcasting

## Requirements

- Go 1.20+
- MySQL
- Redis

## Quick Start

1. Clone the repository
2. Copy `conf/conf_example.yaml` to `conf/conf_test.yaml` and configure it
3. Run the service (specify environment):
   ```bash
   go run main.go -env=testnet  # or mainnet, loc
   ```

## Configuration

Edit `conf/conf_*.yaml` files to configure:
- Database connection
- Redis connection
- External service endpoints
- Platform service fees

## API Documentation

After starting the service, visit `/swagger/index.html` to view the Swagger API documentation.

## License

See the LICENSE file.
