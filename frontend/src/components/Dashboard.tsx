import { usePolling } from '../hooks/usePolling';
import { getOverview } from '../api/client';

export function Dashboard() {
  const { data: overview, loading, error } = usePolling(getOverview, 5000);

  if (loading && !overview) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-4">
        <p className="text-red-800">Error loading overview: {error.message}</p>
      </div>
    );
  }

  if (!overview) return null;

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(value);
  };

  const formatPercent = (value: number) => {
    return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {/* Portfolio Value */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-sm font-medium text-gray-500 mb-1">Portfolio Value</div>
        <div className="text-3xl font-bold text-gray-900">
          {formatCurrency(overview.portfolio_value)}
        </div>
      </div>

      {/* Daily P&L */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-sm font-medium text-gray-500 mb-1">Daily P&L</div>
        <div
          className={`text-3xl font-bold ${
            overview.daily_pnl >= 0 ? 'text-green-600' : 'text-red-600'
          }`}
        >
          {formatCurrency(overview.daily_pnl)}
        </div>
        <div
          className={`text-sm ${
            overview.daily_pnl >= 0 ? 'text-green-600' : 'text-red-600'
          }`}
        >
          {formatPercent((overview.daily_pnl / overview.portfolio_value) * 100)}
        </div>
      </div>

      {/* Total P&L */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-sm font-medium text-gray-500 mb-1">Total P&L</div>
        <div
          className={`text-3xl font-bold ${
            overview.total_pnl >= 0 ? 'text-green-600' : 'text-red-600'
          }`}
        >
          {formatCurrency(overview.total_pnl)}
        </div>
      </div>

      {/* Open Positions */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-sm font-medium text-gray-500 mb-1">Open Positions</div>
        <div className="text-3xl font-bold text-gray-900">{overview.open_positions}</div>
      </div>

      {/* Total Trades */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-sm font-medium text-gray-500 mb-1">Total Trades</div>
        <div className="text-3xl font-bold text-gray-900">{overview.total_trades}</div>
      </div>

      {/* Win Rate */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-sm font-medium text-gray-500 mb-1">Win Rate</div>
        <div className="text-3xl font-bold text-gray-900">
          {overview.win_rate.toFixed(1)}%
        </div>
      </div>
    </div>
  );
}

