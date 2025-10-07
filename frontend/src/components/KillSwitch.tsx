import { useState } from 'react';
import { usePolling } from '../hooks/usePolling';
import { getKillSwitchStatus, enableKillSwitch, disableKillSwitch } from '../api/client';

export function KillSwitch() {
  const { data: status, loading, refetch } = usePolling(getKillSwitchStatus, 3000);
  const [actionLoading, setActionLoading] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [reason, setReason] = useState('');

  const handleEnable = async () => {
    if (!reason.trim()) {
      alert('Please provide a reason for enabling the kill switch');
      return;
    }

    setActionLoading(true);
    try {
      await enableKillSwitch(reason);
      await refetch();
      setShowConfirm(false);
      setReason('');
    } catch (error) {
      alert('Failed to enable kill switch: ' + (error as Error).message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDisable = async () => {
    if (!confirm('Are you sure you want to disable the kill switch and resume trading?')) {
      return;
    }

    setActionLoading(true);
    try {
      await disableKillSwitch();
      await refetch();
    } catch (error) {
      alert('Failed to disable kill switch: ' + (error as Error).message);
    } finally {
      setActionLoading(false);
    }
  };

  if (loading && !status) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  if (!status) return null;

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold text-gray-900">Emergency Kill Switch</h2>
        <div
          className={`px-3 py-1 rounded-full text-sm font-medium ${
            status.enabled
              ? 'bg-red-100 text-red-800'
              : 'bg-green-100 text-green-800'
          }`}
        >
          {status.enabled ? 'ENABLED' : 'Disabled'}
        </div>
      </div>

      {status.enabled && status.reason && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded">
          <div className="text-sm font-medium text-red-800 mb-1">Reason:</div>
          <div className="text-sm text-red-700">{status.reason}</div>
          {status.timestamp && (
            <div className="text-xs text-red-600 mt-1">
              Enabled at: {new Date(status.timestamp).toLocaleString()}
            </div>
          )}
        </div>
      )}

      <div className="text-sm text-gray-600 mb-4">
        The kill switch immediately stops all trading activity, cancels open orders, and prevents
        new trades from being placed.
      </div>

      {!status.enabled ? (
        showConfirm ? (
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Reason for emergency stop:
              </label>
              <input
                type="text"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-red-500"
                placeholder="e.g., Suspicious market activity"
                autoFocus
              />
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleEnable}
                disabled={actionLoading}
                className="flex-1 bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {actionLoading ? 'Enabling...' : 'Confirm Enable'}
              </button>
              <button
                onClick={() => {
                  setShowConfirm(false);
                  setReason('');
                }}
                className="flex-1 bg-gray-300 hover:bg-gray-400 text-gray-800 font-bold py-2 px-4 rounded"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <button
            onClick={() => setShowConfirm(true)}
            className="w-full bg-red-600 hover:bg-red-700 text-white font-bold py-3 px-4 rounded text-lg"
          >
            ENABLE KILL SWITCH
          </button>
        )
      ) : (
        <button
          onClick={handleDisable}
          disabled={actionLoading}
          className="w-full bg-green-600 hover:bg-green-700 text-white font-bold py-3 px-4 rounded text-lg disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {actionLoading ? 'Disabling...' : 'Disable Kill Switch'}
        </button>
      )}
    </div>
  );
}

