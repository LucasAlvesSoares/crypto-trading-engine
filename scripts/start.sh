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
    cd backend && go run ./cmd/migrate up && cd ..
else
    echo "✅ Database already initialized"
fi

echo ""
echo "✅ Infrastructure is ready!"
echo ""
echo "To start the backend services, run:"
echo ""
echo "  # Terminal 1 - Market Data Service"
echo "  cd backend && go run ./cmd/market-data"
echo ""
echo "  # Terminal 2 - Trading Bot"
echo "  cd backend && go run ./cmd/trading-bot"
echo ""
echo "  # Terminal 3 - API Gateway"
echo "  cd backend && go run ./cmd/api-gateway"
echo ""
echo "Or use the Makefile:"
echo ""
echo "  cd backend && make run-services"
echo ""
echo "To start the frontend dashboard:"
echo ""
echo "  cd frontend && npm install && npm run dev"
echo ""

