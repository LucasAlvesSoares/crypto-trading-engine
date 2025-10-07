import { useState } from 'react';
import { usePolling } from '../hooks/usePolling';
import { getStrategy, toggleStrategy } from '../api/client';

export function StrategyControl() {
  const { data: strategy, loading, refetch } = usePolling(getStrategy, 5000);
  const [actionLoading, setActionLoading] = useState(false);

  const handleToggle = async () => {
    if (!strategy) return;

    const action = strategy.is_active ? 'disable' : 'enable';
    if (!confirm(`Are you sure you want to ${action} the trading strategy?`)) {
      return;
    }

    setActionLoading(true);
    try {
      await toggleStrategy(!strategy.is_active);
      await refetch();
    } catch (error) {
      alert(`Failed to ${action} strategy: ` + (error as Error).message);
    } finally {
      setActionLoading(false);
    }
  };

  if (loading && !strategy) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-gray-500">Loading strategy...</div>
      </div>
    );
  }

  if (!strategy) return null;

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold text-gray-900">Strategy Control</h2>
        <div
          className={`px-3 py-1 rounded-full text-sm font-medium ${
            strategy.is_active
              ? 'bg-green-100 text-green-800'
              : 'bg-gray-100 text-gray-800'
          }`}
        >
          {strategy.is_active ? '▶ ACTIVE' : '⏸ INACTIVE'}
        </div>
      </div>

      <div className="space-y-3 mb-4">
        <div>
          <span className="text-sm font-medium text-gray-500">Name:</span>
          <span className="ml-2 text-sm text-gray-900">{strategy.name}</span>
        </div>
        <div>
          <span className="text-sm font-medium text-gray-500">Type:</span>
          <span className="ml-2 text-sm text-gray-900">{strategy.type}</span>
        </div>
      </div>

      {strategy.config && (
        <div className="mb-4 p-3 bg-gray-50 rounded">
          <div className="text-sm font-medium text-gray-700 mb-2">Configuration:</div>
          <div className="text-xs text-gray-600 space-y-1">
            {Object.entries(strategy.config).map(([key, value]) => (
              <div key={key}>
                <span className="font-medium">{key}:</span> {JSON.stringify(value)}
              </div>
            ))}
          </div>
        </div>
      )}

      <button
        onClick={handleToggle}
        disabled={actionLoading}
        className={`w-full font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed ${
          strategy.is_active
            ? 'bg-red-600 hover:bg-red-700 text-white'
            : 'bg-green-600 hover:bg-green-700 text-white'
        }`}
      >
        {actionLoading
          ? 'Processing...'
          : strategy.is_active
          ? '⏸ Disable Strategy'
          : '▶ Enable Strategy'}
      </button>

      {!strategy.is_active && (
        <div className="mt-3 text-sm text-yellow-700 bg-yellow-50 border border-yellow-200 rounded p-3">
          ⚠️ Strategy is currently disabled. No trades will be placed.
        </div>
      )}
    </div>
  );
}

