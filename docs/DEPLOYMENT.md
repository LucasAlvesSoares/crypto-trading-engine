# Production Deployment Guide

This guide covers deploying the crypto trading bot to a production environment.

## Prerequisites

- Linux server (Ubuntu 22.04 LTS recommended)
- 2GB+ RAM
- 20GB+ disk space
- Docker & Docker Compose
- Domain name (optional, for HTTPS)
- Coinbase Advanced Trade API credentials

## Security Checklist

Before deploying to production:

- [ ] Store all secrets in environment variables, never in code
- [ ] Use strong passwords for PostgreSQL
- [ ] Enable SSL/TLS for database connections
- [ ] Run services as non-root user
- [ ] Set up firewall rules (UFW or iptables)
- [ ] Enable fail2ban for SSH protection
- [ ] Keep Docker images up to date
- [ ] Monitor disk space and logs
- [ ] Set up automated backups
- [ ] Test kill switch functionality

## Infrastructure Setup

### 1. Server Preparation

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo apt install docker-compose-plugin -y

# Create application directory
sudo mkdir -p /opt/trading-bot
sudo chown $USER:$USER /opt/trading-bot
cd /opt/trading-bot
```

### 2. Clone Repository

```bash
git clone <your-repository-url> .
```

### 3. Environment Configuration

Create production environment file:

```bash
cp backend/config.sample.env backend/.env
chmod 600 backend/.env  # Restrict permissions
```

Edit `backend/.env` with production values:

```env
# Database (use strong password)
DATABASE_URL=postgresql://trading:STRONG_PASSWORD_HERE@postgres:5432/trading_bot?sslmode=require

# NATS
NATS_URL=nats://nats:4222

# Coinbase API
COINBASE_API_KEY=your_production_key
COINBASE_API_SECRET=your_production_secret
COINBASE_API_PASSPHRASE=your_production_passphrase
COINBASE_USE_SANDBOX=false  # Set to true for testing

# Risk Management (CRITICAL - review these carefully)
RISK_MAX_POSITION_SIZE_USD=100
RISK_MAX_OPEN_POSITIONS=1
RISK_DAILY_LOSS_LIMIT_PERCENT=2.0
RISK_STOP_LOSS_PERCENT=2.0
RISK_MAX_HOLD_TIME_HOURS=24

# Trading Mode (START WITH PAPER TRADING)
TRADING_MODE=paper  # Change to 'live' only after thorough testing

# Strategy Parameters
STRATEGY_SYMBOL=BTC-USD
STRATEGY_RSI_PERIOD=14
STRATEGY_RSI_OVERSOLD=30
STRATEGY_RSI_OVERBOUGHT=70
STRATEGY_BB_PERIOD=20
STRATEGY_BB_STD_DEV=2.0
STRATEGY_SMA_PERIOD=50

# System
LOG_LEVEL=info
PORT=8080
```

### 4. Update Docker Compose for Production

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    container_name: trading-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: trading
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: trading_bot
    volumes:
      - postgres-data:/var/lib/postgresql/data
    ports:
      - "127.0.0.1:5432:5432"  # Bind to localhost only
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U trading"]
      interval: 10s
      timeout: 5s
      retries: 5

  nats:
    image: nats:2.10-alpine
    container_name: trading-nats
    restart: unless-stopped
    ports:
      - "127.0.0.1:4222:4222"  # Bind to localhost only
    command: 
      - "--jetstream"
      - "--store_dir=/data"
    volumes:
      - nats-data:/data
    healthcheck:
      test: ["CMD", "nc", "-z", "localhost", "4222"]
      interval: 10s
      timeout: 5s
      retries: 5

  market-data:
    build:
      context: .
      dockerfile: backend/Dockerfile
      target: market-data
    container_name: trading-market-data
    restart: unless-stopped
    env_file:
      - backend/.env
    depends_on:
      postgres:
        condition: service_healthy
      nats:
        condition: service_healthy
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  trading-bot:
    build:
      context: .
      dockerfile: backend/Dockerfile
      target: trading-bot
    container_name: trading-bot
    restart: unless-stopped
    env_file:
      - backend/.env
    depends_on:
      postgres:
        condition: service_healthy
      nats:
        condition: service_healthy
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  api-gateway:
    build:
      context: .
      dockerfile: backend/Dockerfile
      target: api-gateway
    container_name: trading-api
    restart: unless-stopped
    ports:
      - "8080:8080"
    env_file:
      - backend/.env
    depends_on:
      postgres:
        condition: service_healthy
      nats:
        condition: service_healthy
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    container_name: trading-frontend
    restart: unless-stopped
    ports:
      - "3000:80"
    depends_on:
      - api-gateway
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  postgres-data:
  nats-data:
```

### 5. Create Backend Dockerfile

Create `backend/Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -o /market-data ./cmd/market-data && \
    CGO_ENABLED=0 GOOS=linux go build -o /trading-bot ./cmd/trading-bot && \
    CGO_ENABLED=0 GOOS=linux go build -o /api-gateway ./cmd/api-gateway

FROM alpine:3.18 AS market-data
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /market-data /market-data
ENTRYPOINT ["/market-data"]

FROM alpine:3.18 AS trading-bot
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /trading-bot /trading-bot
ENTRYPOINT ["/trading-bot"]

FROM alpine:3.18 AS api-gateway
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /api-gateway /api-gateway
EXPOSE 8080
ENTRYPOINT ["/api-gateway"]

FROM alpine:3.18 AS migrate
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /migrate /migrate
COPY backend/migrations /migrations
ENTRYPOINT ["/migrate"]
```

### 6. Create Frontend Dockerfile

Create `frontend/Dockerfile`:

```dockerfile
FROM node:20-alpine AS builder

WORKDIR /app
COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

Create `frontend/nginx.conf`:

```nginx
server {
    listen 80;
    server_name localhost;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api {
        proxy_pass http://api-gateway:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

## Deployment Steps

### 1. Build and Start Services

```bash
# Build images
docker compose -f docker-compose.prod.yml build

# Start infrastructure
docker compose -f docker-compose.prod.yml up -d postgres nats

# Wait for services to be healthy
sleep 10

# Run migrations
docker compose -f docker-compose.prod.yml run --rm migrate up

# Start all services
docker compose -f docker-compose.prod.yml up -d
```

### 2. Verify Deployment

```bash
# Check service status
docker compose -f docker-compose.prod.yml ps

# Check logs
docker compose -f docker-compose.prod.yml logs -f

# Test health endpoints
curl http://localhost:8080/health

# Access dashboard
curl http://localhost:3000
```

### 3. Test Kill Switch

Before enabling live trading, verify the kill switch works:

1. Open dashboard at http://localhost:3000
2. Click the kill switch button
3. Verify all trading stops
4. Check logs for kill switch activation
5. Test re-enabling

## Monitoring

### View Logs

```bash
# All services
docker compose -f docker-compose.prod.yml logs -f

# Specific service
docker compose -f docker-compose.prod.yml logs -f trading-bot

# Last 100 lines
docker compose -f docker-compose.prod.yml logs --tail=100
```

### Check Resource Usage

```bash
docker stats
```

### Database Backup

```bash
# Create backup
docker exec trading-postgres pg_dump -U trading trading_bot > backup_$(date +%Y%m%d_%H%M%S).sql

# Restore backup
docker exec -i trading-postgres psql -U trading trading_bot < backup_20250101_120000.sql
```

## Maintenance

### Update Application

```bash
# Pull latest changes
git pull

# Rebuild and restart
docker compose -f docker-compose.prod.yml up -d --build
```

### Restart Services

```bash
# All services
docker compose -f docker-compose.prod.yml restart

# Specific service
docker compose -f docker-compose.prod.yml restart trading-bot
```

### Clean Up

```bash
# Remove old images
docker image prune -a

# Remove old logs
docker compose -f docker-compose.prod.yml logs --tail=0
```

## Firewall Configuration

```bash
# Allow SSH
sudo ufw allow 22/tcp

# Allow HTTP/HTTPS (if using reverse proxy)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Enable firewall
sudo ufw enable

# Check status
sudo ufw status
```

## HTTPS Setup (Optional)

If exposing the dashboard publicly:

### Using Nginx + Let's Encrypt

```bash
# Install nginx and certbot
sudo apt install nginx certbot python3-certbot-nginx

# Get SSL certificate
sudo certbot --nginx -d yourdomain.com

# Configure nginx reverse proxy
sudo nano /etc/nginx/sites-available/trading-bot
```

Example nginx config:

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }
}
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
docker compose -f docker-compose.prod.yml logs service-name

# Check environment variables
docker compose -f docker-compose.prod.yml config

# Verify database connection
docker exec trading-postgres psql -U trading -c "SELECT 1"
```

### High Memory Usage

```bash
# Check container stats
docker stats

# Restart service
docker compose -f docker-compose.prod.yml restart trading-bot
```

### Database Connection Issues

```bash
# Check PostgreSQL is running
docker compose -f docker-compose.prod.yml ps postgres

# Test connection
docker exec trading-postgres psql -U trading -c "\l"
```

## Emergency Procedures

### Immediate Shutdown

```bash
# Stop all trading immediately
curl -X POST http://localhost:8080/api/v1/system/kill-switch

# Or stop all services
docker compose -f docker-compose.prod.yml stop
```

### Recover from Crash

```bash
# Check what's running
docker compose -f docker-compose.prod.yml ps

# Restart failed services
docker compose -f docker-compose.prod.yml up -d

# Check for data corruption
docker exec trading-postgres psql -U trading trading_bot -c "SELECT COUNT(*) FROM orders"
```

## Production Checklist

Before going live with real money:

- [ ] Tested extensively in paper trading mode (minimum 1 week)
- [ ] Reviewed all risk management parameters
- [ ] Verified kill switch functionality
- [ ] Set up monitoring and alerts
- [ ] Configured automated backups
- [ ] Documented incident response procedures
- [ ] Tested with minimal position sizes first
- [ ] Verified API credentials are correct
- [ ] Checked account has sufficient balance
- [ ] Set up log rotation
- [ ] Reviewed and understood all code
- [ ] Have manual intervention plan ready

## Support

For issues or questions:
- Review logs: `docker compose -f docker-compose.prod.yml logs -f`
- Check health endpoints
- Review configuration files
- Consult the main README.md

## Disclaimer

⚠️ **Trading cryptocurrencies involves significant financial risk. This software is provided as-is with no guarantees. Always start with paper trading and never invest more than you can afford to lose.**

