# Crypto Trading Bot

A production-ready cryptocurrency trading bot built with Go and React, featuring paper trading, backtesting, and comprehensive risk management.

## Features

- **Paper Trading**: Test strategies with simulated execution before risking real money
- **Single Exchange Integration**: Coinbase Advanced Trade API with comprehensive error handling
- **Mean Reversion Strategy**: RSI + Bollinger Bands + SMA indicators
- **Risk Management**: Kill switch, position limits, daily loss limits, stop-loss
- **Backtesting Engine**: Historical simulation with realistic slippage and fees
- **Real-Time Dashboard**: React + TypeScript UI with live charts and monitoring
- **Event-Driven Architecture**: NATS message bus for decoupled components
- **Production-Ready**: Observability, fault tolerance, comprehensive testing

## Architecture

```
┌──────────────┐
│   Dashboard  │ (React + TypeScript)
└──────┬───────┘
       │ HTTP/WebSocket
┌──────▼────────────────┐
│   API Gateway (Gin)   │
└──────┬────────────────┘
       │
   ┌───▼────┐
   │  NATS  │ (Message Bus)
   └───┬────┘
       │
   ┌───┴────────────────────────────────┐
   │                                    │
┌──▼──────────────┐        ┌───────────▼────────┐
│ Market Data Svc │        │  Strategy Engine   │
└─────────────────┘        └────────┬───────────┘
                                    │
                        ┌───────────▼──────────┐
                        │   Risk Manager       │
                        └───────────┬──────────┘
                                    │
                        ┌───────────▼──────────┐
                        │  Order Manager       │
                        └───────────┬──────────┘
                                    │
                        ┌───────────▼──────────┐
                        │  Exchange Connector  │
                        └───────────┬──────────┘
                                    │
                        ┌───────────▼──────────┐
                        │     PostgreSQL       │
                        └──────────────────────┘
```

## Tech Stack

**Backend:**
- Go 1.21+
- PostgreSQL 15+
- NATS 2.10+
- Gin (HTTP framework)
- sqlc (type-safe SQL)

**Frontend:**
- React 18
- TypeScript 5
- TradingView Lightweight Charts
- TailwindCSS

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+
- Node.js 18+
- Make

### 1. Start Infrastructure
```bash
docker-compose up -d
```

### 2. Run Database Migrations
```bash
make migrate-up
```

### 3. Start Backend Services
```bash
make run-services
```

### 4. Start Frontend
```bash
cd frontend
npm install
npm start
```

### 5. Access Dashboard
Open http://localhost:3000

## Configuration

Copy `.env.example` to `.env` and configure:

```env
# Database
DATABASE_URL=postgresql://trading:trading@localhost:5432/trading_bot?sslmode=disable

# NATS
NATS_URL=nats://localhost:4222

# Coinbase API
COINBASE_API_KEY=your_api_key
COINBASE_API_SECRET=your_api_secret
COINBASE_API_PASSPHRASE=your_passphrase
COINBASE_USE_SANDBOX=true

# Risk Management
RISK_MAX_POSITION_SIZE_USD=100
RISK_MAX_OPEN_POSITIONS=1
RISK_DAILY_LOSS_LIMIT_PERCENT=2.0
RISK_STOP_LOSS_PERCENT=2.0
RISK_MAX_HOLD_TIME_HOURS=24

# Trading Mode
TRADING_MODE=paper  # paper or live
```

## Project Structure

```
.
├── cmd/
│   ├── api-gateway/       # HTTP API and WebSocket server
│   ├── market-data/       # Price data ingestion
│   ├── strategy-engine/   # Trading strategy execution
│   ├── risk-manager/      # Risk validation service
│   ├── order-manager/     # Order execution service
│   └── backtest/          # Backtesting CLI tool
├── internal/
│   ├── exchange/          # Exchange connector implementations
│   ├── strategy/          # Trading strategies
│   ├── risk/              # Risk management logic
│   ├── models/            # Domain models
│   ├── database/          # Database layer (sqlc generated)
│   └── events/            # NATS event definitions
├── frontend/              # React dashboard
├── migrations/            # Database migrations
├── docker-compose.yml     # Infrastructure setup
└── Makefile              # Build and run commands
```

## Development

### Run Tests
```bash
make test
```

### Run Linter
```bash
make lint
```

### Generate Database Code
```bash
make sqlc-generate
```

### Build All Services
```bash
make build
```

## Safety Features

### Kill Switch
- Prominent red button in UI
- API endpoint: `POST /api/v1/system/kill-switch`
- Immediately halts all trading and cancels orders
- Requires manual re-enable

### Risk Limits
- Maximum position size (default: $100)
- Maximum open positions (default: 1)
- Daily loss limit (default: 2%)
- Per-trade stop-loss (default: 2%)
- Maximum hold time (default: 24 hours)

### Paper Trading
- Identical code path to live trading
- Simulated execution with realistic slippage
- No real money at risk
- Always start here before going live

## Backtesting

Run historical backtests:

```bash
./bin/backtest \
  --strategy mean-reversion \
  --symbol BTC-USD \
  --start 2024-01-01 \
  --end 2024-12-31 \
  --initial-balance 10000
```

## Monitoring

- **Logs**: Structured JSON logs to stdout
- **Health Checks**: `/health` endpoint on each service
- **Metrics**: Prometheus metrics at `/metrics`
- **Dashboard**: Real-time monitoring at http://localhost:3000

## Production Deployment

See [DEPLOYMENT.md](./docs/DEPLOYMENT.md) for production deployment guide.

## License

MIT License - See LICENSE file for details

## Disclaimer

**⚠️ USE AT YOUR OWN RISK ⚠️**

This software is for educational purposes. Cryptocurrency trading carries significant financial risk. Always:
- Start with paper trading
- Never invest more than you can afford to lose
- Thoroughly test strategies before live trading
- Monitor positions actively
- Understand the code before running it

The authors are not responsible for any financial losses incurred.

