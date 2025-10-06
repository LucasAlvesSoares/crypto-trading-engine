#!/bin/bash

set -e

echo "🚀 Starting Crypto Trading Bot..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Start infrastructure
echo "📦 Starting infrastructure (PostgreSQL, NATS)..."
docker-compose up -d

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 5

# Check if database needs initialization
if ! docker exec trading-bot-postgres psql -U trading -d trading_bot -c '\dt' > /dev/null 2>&1; then
    echo "🔧 Running database migrations..."
    go run ./cmd/migrate up
else
    echo "✅ Database already initialized"
fi

echo ""
echo "✅ Infrastructure is ready!"
echo ""
echo "To start the trading bot services, run:"
echo ""
echo "  # Terminal 1 - Market Data Service"
echo "  go run ./cmd/market-data"
echo ""
echo "  # Terminal 2 - Trading Bot"
echo "  go run ./cmd/trading-bot"
echo ""
echo "  # Terminal 3 - API Gateway"
echo "  go run ./cmd/api-gateway"
echo ""
echo "Or use the Makefile:"
echo ""
echo "  make run-services"
echo ""

