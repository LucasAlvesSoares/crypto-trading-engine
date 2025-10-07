# Quick Start Guide

Get the trading bot running in 5 minutes!

## Prerequisites

- Docker & Docker Compose
- Go 1.21+
- Node.js 18+ and npm
- Make (optional)

## Step 1: Clone and Setup

```bash
cd /path/to/crypto-trading-bot

# Copy sample environment config
cp backend/config.sample.env backend/.env

# The defaults are fine for initial testing (paper trading mode)
```

## Step 2: Start Infrastructure

```bash
# Start PostgreSQL and NATS
docker-compose up -d

# Wait a few seconds for services to start
sleep 5
```

## Step 3: Run Database Migrations

```bash
cd backend
go run ./cmd/migrate up
```

You should see:
```
Running migrations up...
Migration completed successfully!
```

## Step 4: Start Backend Services

Open 3 terminal windows:

### Terminal 1: Market Data Service
```bash
cd backend
go run ./cmd/market-data
```

This will:
- Connect to the paper exchange
- Start simulating BTC-USD price movements around $45,000
- Ingest and store price data

### Terminal 2: Trading Bot
```bash
cd backend
go run ./cmd/trading-bot
```

This will:
- Load the mean reversion strategy
- Subscribe to price updates
- Generate trading signals (when enabled)

### Terminal 3: API Gateway
```bash
cd backend
go run ./cmd/api-gateway
```

This provides the REST API at `http://localhost:8080`

## Step 5: Start the Frontend Dashboard

Open a 4th terminal window:

```bash
cd frontend

# Install dependencies (first time only)
npm install

# Start the development server
npm run dev
```

You should see:
```
  VITE v5.0.8  ready in 500 ms

  ➜  Local:   http://localhost:3000/
  ➜  Network: use --host to expose
```

**Open your browser to http://localhost:3000** 🎉

You'll see the trading bot dashboard with:
- 📊 **Portfolio Overview** - Real-time portfolio value, P&L, win rate
- 🛑 **Kill Switch** - Emergency stop button (try it!)
- ⚙️ **Strategy Control** - Enable/disable trading with one click
- 📈 **Recent Trades** - Live trade history with entry/exit prices and P&L

The dashboard auto-refreshes every 5 seconds, so you'll see live updates as trades happen!

## Step 6: Test the System

### Option A: Use the Dashboard (Recommended)

1. **Enable the strategy** - Click the "Enable Strategy" button
2. **Watch for signals** - Wait for the bot to detect RSI < 30 (may take a few minutes)
3. **Monitor trades** - See trades appear in the table
4. **Test kill switch** - Click the red "Enable Kill Switch" button to stop all trading

### Option B: Use the API Directly

### Check API Health
```bash
curl http://localhost:8080/health
```

### View Overview
```bash
curl http://localhost:8080/api/v1/overview
```

### Enable Trading Strategy
```bash
curl -X POST http://localhost:8080/api/v1/strategy/toggle \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

### Check Open Trades
```bash
curl http://localhost:8080/api/v1/trades
```

### Test Kill Switch
```bash
# Enable
curl -X POST http://localhost:8080/api/v1/kill-switch/enable \
  -H "Content-Type: application/json" \
  -d '{"reason": "Testing emergency stop"}'

# Disable
curl -X POST http://localhost:8080/api/v1/kill-switch/disable \
  -H "Content-Type: application/json"
```

## What's Happening?

1. **Market Data Service** is simulating BTC price movements around $45,000
2. **Trading Bot** is watching for mean reversion signals:
   - LONG when RSI < 30 AND price < lower Bollinger Band
   - EXIT when price crosses 20-period SMA
3. **Risk Manager** is enforcing limits:
   - Max position: $100
   - Max open positions: 1
   - Daily loss limit: 2%
   - Stop-loss: 2%
4. **Paper Exchange** is simulating order execution with realistic slippage

## Configuration

Edit `backend/.env` to customize:

```env
# Enable/disable strategy
STRATEGY_ENABLED=true

# Trading symbol
STRATEGY_SYMBOL=BTC-USD

# Risk parameters
RISK_MAX_POSITION_SIZE_USD=100
RISK_DAILY_LOSS_LIMIT_PERCENT=2.0
RISK_STOP_LOSS_PERCENT=2.0
```

## Monitoring

### View Logs
All services log to stdout. Watch for:
- Price updates (every second)
- Indicator calculations (when enough data)
- Trade signals (when conditions met)
- Order execution
- P&L updates

### Database Queries
```bash
# Connect to database
docker exec -it trading-bot-postgres psql -U trading -d trading_bot

# View recent trades
SELECT * FROM trades ORDER BY entry_time DESC LIMIT 10;

# View balances
SELECT * FROM balances;

# View risk events
SELECT * FROM risk_events ORDER BY timestamp DESC LIMIT 10;
```

## Troubleshooting

### "Database connection failed"
- Make sure Docker is running
- Check: `docker ps` (you should see postgres and nats containers)
- Wait a few more seconds and try again

### "No price data"
- The market data service needs ~20 seconds to build price history
- Check the market-data service logs for "Price update processed"

### "Strategy not generating signals"
- Make sure strategy is enabled (use dashboard or set `STRATEGY_ENABLED=true` in `backend/.env`)
- The strategy needs 20+ price points to calculate indicators
- Mean reversion signals are rare - wait for RSI < 30

### "Frontend won't load" or "Connection refused"
- Make sure the API Gateway is running on port 8080
- Check `cd backend && go run ./cmd/api-gateway` is running
- The frontend proxies API requests to localhost:8080

### "npm install fails"
- Clear npm cache: `npm cache clean --force`
- Delete `node_modules` and try again: `rm -rf frontend/node_modules && cd frontend && npm install`

### "Kill switch is enabled"
- This happens if daily loss limit is exceeded
- Disable via API: `curl -X POST http://localhost:8080/api/v1/kill-switch/disable`

## Next Steps

1. **Watch it trade**: Let it run for 30-60 minutes to see the full cycle
2. **Tune parameters**: Adjust RSI thresholds, position sizes, etc. in `backend/.env`
3. **Customize the dashboard**: Edit React components in `frontend/src/components/`
4. **Add strategies**: Implement your own in `backend/internal/strategy/`
5. **Backtest**: Create historical price data and test strategies

## Safety Reminders

✅ This is **PAPER TRADING** by default - no real money!

⚠️ Before going live:
- Thoroughly test all strategies
- Understand the code completely
- Start with tiny position sizes
- Monitor constantly
- Use the kill switch liberally

## Stopping the System

```bash
# Stop all services (Ctrl+C in each terminal):
# - Market Data Service
# - Trading Bot
# - API Gateway
# - Frontend (npm run dev)

# Stop infrastructure
docker-compose down

# (Optional) Remove all data
docker-compose down -v
```

## Support

- Check logs for errors in each terminal
- Review backend code in `backend/internal/` and `backend/cmd/`
- Review frontend code in `frontend/src/`
- Test components individually
- Use the kill switch if anything looks wrong!
- Open browser DevTools (F12) to see frontend errors

