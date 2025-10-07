-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Exchanges table
CREATE TABLE exchanges (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    api_secret_encrypted TEXT NOT NULL,
    api_passphrase_encrypted TEXT,
    is_paper_trading BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_exchanges_is_active ON exchanges(is_active);

-- Strategies table
CREATE TABLE strategies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    config JSONB NOT NULL,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_strategies_is_active ON strategies(is_active);
CREATE INDEX idx_strategies_type ON strategies(type);

-- Orders table (authoritative source of truth)
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_order_id TEXT UNIQUE NOT NULL,
    exchange_order_id TEXT,
    exchange_id UUID REFERENCES exchanges(id),
    strategy_id UUID REFERENCES strategies(id),
    symbol TEXT NOT NULL,
    side TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
    type TEXT NOT NULL CHECK (type IN ('MARKET', 'LIMIT')),
    quantity DECIMAL(20,8) NOT NULL,
    price DECIMAL(20,8),
    stop_loss_price DECIMAL(20,8),
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'OPEN', 'FILLED', 'CANCELLED', 'FAILED')),
    filled_quantity DECIMAL(20,8) DEFAULT 0,
    average_fill_price DECIMAL(20,8),
    fees DECIMAL(20,8) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    filled_at TIMESTAMPTZ
);

CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_client_order_id ON orders(client_order_id);
CREATE INDEX idx_orders_exchange_order_id ON orders(exchange_order_id);
CREATE INDEX idx_orders_strategy_id ON orders(strategy_id);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

-- Trades table (executed positions)
CREATE TABLE trades (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entry_order_id UUID REFERENCES orders(id),
    exit_order_id UUID REFERENCES orders(id),
    strategy_id UUID REFERENCES strategies(id),
    symbol TEXT NOT NULL,
    entry_price DECIMAL(20,8) NOT NULL,
    exit_price DECIMAL(20,8),
    quantity DECIMAL(20,8) NOT NULL,
    side TEXT NOT NULL CHECK (side IN ('BUY', 'SELL', 'LONG', 'SHORT')),
    entry_time TIMESTAMPTZ NOT NULL,
    exit_time TIMESTAMPTZ,
    pnl DECIMAL(20,8),
    pnl_percent DECIMAL(10,4),
    fees_total DECIMAL(20,8),
    hold_duration INTERVAL,
    exit_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_trades_strategy_id ON trades(strategy_id);
CREATE INDEX idx_trades_entry_time ON trades(entry_time DESC);
CREATE INDEX idx_trades_symbol ON trades(symbol);
CREATE INDEX idx_trades_exit_time ON trades(exit_time DESC) WHERE exit_time IS NOT NULL;

-- Price data table (regular PostgreSQL table, no TimescaleDB)
CREATE TABLE price_data (
    id BIGSERIAL PRIMARY KEY,
    time TIMESTAMPTZ NOT NULL,
    exchange TEXT NOT NULL,
    symbol TEXT NOT NULL,
    open DECIMAL(20,8) NOT NULL,
    high DECIMAL(20,8) NOT NULL,
    low DECIMAL(20,8) NOT NULL,
    close DECIMAL(20,8) NOT NULL,
    volume DECIMAL(20,8) NOT NULL,
    interval TEXT NOT NULL, -- '1m', '5m', '1h', etc.
    UNIQUE(time, exchange, symbol, interval)
);

CREATE INDEX idx_price_data_symbol_time ON price_data(symbol, time DESC);
CREATE INDEX idx_price_data_exchange_symbol ON price_data(exchange, symbol);

-- Risk events table (audit trail)
CREATE TABLE risk_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    strategy_id UUID REFERENCES strategies(id),
    event_type TEXT NOT NULL,
    description TEXT NOT NULL,
    action_taken TEXT NOT NULL,
    metadata JSONB,
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_risk_events_timestamp ON risk_events(timestamp DESC);
CREATE INDEX idx_risk_events_strategy_id ON risk_events(strategy_id);
CREATE INDEX idx_risk_events_event_type ON risk_events(event_type);

-- System logs table
CREATE TABLE logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    level TEXT NOT NULL CHECK (level IN ('DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL')),
    component TEXT NOT NULL,
    message TEXT NOT NULL,
    metadata JSONB,
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_logs_timestamp ON logs(timestamp DESC);
CREATE INDEX idx_logs_level ON logs(level);
CREATE INDEX idx_logs_component ON logs(component);

-- Performance snapshots table (hourly aggregations)
CREATE TABLE performance_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    strategy_id UUID REFERENCES strategies(id),
    portfolio_value DECIMAL(20,2) NOT NULL,
    cash_balance DECIMAL(20,2) NOT NULL,
    total_pnl DECIMAL(20,2) NOT NULL,
    daily_pnl DECIMAL(20,2) NOT NULL,
    open_positions INT NOT NULL,
    total_trades INT NOT NULL,
    win_rate DECIMAL(5,2),
    sharpe_ratio DECIMAL(10,4),
    max_drawdown DECIMAL(5,2),
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_performance_snapshots_timestamp ON performance_snapshots(timestamp DESC);
CREATE INDEX idx_performance_snapshots_strategy_id ON performance_snapshots(strategy_id);

-- System configuration table
CREATE TABLE system_config (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default kill switch state
INSERT INTO system_config (key, value) VALUES ('kill_switch', '{"enabled": false, "reason": null, "timestamp": null}');

-- Balances table (for paper trading)
CREATE TABLE balances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    exchange_id UUID REFERENCES exchanges(id),
    currency TEXT NOT NULL,
    available DECIMAL(20,8) NOT NULL,
    locked DECIMAL(20,8) DEFAULT 0,
    total DECIMAL(20,8) GENERATED ALWAYS AS (available + locked) STORED,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(exchange_id, currency)
);

CREATE INDEX idx_balances_exchange_id ON balances(exchange_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_exchanges_updated_at BEFORE UPDATE ON exchanges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_strategies_updated_at BEFORE UPDATE ON strategies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_balances_updated_at BEFORE UPDATE ON balances
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

