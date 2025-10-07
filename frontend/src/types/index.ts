export interface Overview {
  portfolio_value: number;
  daily_pnl: number;
  total_pnl: number;
  open_positions: number;
  total_trades: number;
  win_rate: number;
}

export interface Trade {
  id: string;
  symbol: string;
  side: string;
  entry_price: string;
  exit_price?: string;
  quantity: string;
  pnl?: number;
  pnl_percent?: number;
  entry_time: string;
  exit_time?: string;
  exit_reason?: string;
  status: 'open' | 'closed';
}

export interface Order {
  id: string;
  symbol: string;
  side: string;
  type: string;
  quantity: string;
  status: string;
  created_at: string;
}

export interface Balance {
  currency: string;
  available: number;
  locked: number;
  total: number;
}

export interface Strategy {
  id: string;
  name: string;
  type: string;
  is_active: boolean;
  config: Record<string, any>;
}

export interface KillSwitchStatus {
  enabled: boolean;
  reason?: string;
  timestamp?: string;
}

export interface RiskEvent {
  event_type: string;
  description: string;
  action_taken: string;
  timestamp: string;
}

export interface Log {
  level: string;
  component: string;
  message: string;
  timestamp: string;
}

