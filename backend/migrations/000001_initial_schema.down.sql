-- Drop triggers
DROP TRIGGER IF EXISTS update_balances_updated_at ON balances;
DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
DROP TRIGGER IF EXISTS update_strategies_updated_at ON strategies;
DROP TRIGGER IF EXISTS update_exchanges_updated_at ON exchanges;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (in reverse order of creation)
DROP TABLE IF EXISTS balances CASCADE;
DROP TABLE IF EXISTS system_config CASCADE;
DROP TABLE IF EXISTS performance_snapshots CASCADE;
DROP TABLE IF EXISTS logs CASCADE;
DROP TABLE IF EXISTS risk_events CASCADE;
DROP TABLE IF EXISTS price_data CASCADE;
DROP TABLE IF EXISTS trades CASCADE;
DROP TABLE IF EXISTS orders CASCADE;
DROP TABLE IF EXISTS strategies CASCADE;
DROP TABLE IF EXISTS exchanges CASCADE;

-- Drop extension
DROP EXTENSION IF EXISTS "uuid-ossp";

