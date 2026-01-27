import React, { useState } from 'react';
import { X } from 'lucide-react';
import { httpClient } from '../lib/httpClient';

interface PartialCloseModalProps {
  symbol: string;
  currentQuantity: number;
  traderID: string;
  onClose: () => void;
  onSuccess: () => void;
}

const PartialCloseModal: React.FC<PartialCloseModalProps> = ({
  symbol,
  currentQuantity,
  traderID,
  onClose,
  onSuccess,
}) => {
  const [quantity, setQuantity] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  const panelStyle = {
    background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
    border: '1px solid #2B3139',
    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
  }
  const inputStyle = {
    background: 'rgba(255, 255, 255, 0.05)',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  }
  const dangerButtonStyle = {
    background: 'linear-gradient(135deg, #F6465D 0%, #E52E3D 100%)',
    color: '#FFFFFF',
    fontWeight: '600',
  }
  const quickButtonStyle = {
    background: 'rgba(255, 255, 255, 0.05)',
    color: '#EAECEF',
    border: '1px solid #2B3139',
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    const closeQty = parseFloat(quantity);

    if (!quantity || isNaN(closeQty) || closeQty <= 0) {
      setError('Please enter a valid quantity');
      return;
    }

    if (closeQty > currentQuantity) {
      setError(`Cannot close ${closeQty}, position size is only ${currentQuantity.toFixed(4)}`);
      return;
    }

    setIsSubmitting(true);

    try {
      const result = await httpClient.post<{ error?: string }>(
        `/api/position/close-partial?trader_id=${traderID}`,
        {
          symbol,
          quantity: closeQty,
        }
      );

      if (!result.success) {
        setError(result.message || 'Failed to close position');
        return;
      }

      alert(`Successfully closed ${closeQty} ${symbol}`);
      onSuccess();
    } catch (err) {
      setError(`Error: ${err}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleQuickClose = async (percentage: number) => {
    const qtyToClose = currentQuantity * (percentage / 100);
    setQuantity(qtyToClose.toFixed(4));
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="rounded-lg p-6 max-w-md w-full" style={panelStyle}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>Partial Close</h2>
          <button
            onClick={onClose}
            className="transition-all hover:scale-110 active:scale-95"
            style={{ color: '#848E9C' }}
          >
            <X size={24} />
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 rounded text-sm" style={{ background: 'rgba(246, 70, 93, 0.1)', border: '1px solid rgba(246, 70, 93, 0.5)', color: '#F6465D' }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-2" style={{ color: '#848E9C' }}>
              Symbol: <span className="font-bold" style={{ color: '#EAECEF' }}>{symbol}</span>
            </label>
            <label className="block text-sm font-medium mb-2" style={{ color: '#848E9C' }}>
              Current Position: <span className="font-bold" style={{ color: '#0ECB81' }}>{currentQuantity.toFixed(4)}</span>
            </label>
          </div>

          <div>
            <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
              Close Quantity
            </label>
            <input
              type="number"
              step="0.0001"
              min="0"
              max={currentQuantity}
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              placeholder={`Max: ${currentQuantity.toFixed(4)}`}
              className="w-full px-3 py-2 rounded transition-all"
              style={inputStyle}
            />
          </div>

          {/* Quick close buttons */}
          <div>
            <label className="block text-sm font-medium mb-2" style={{ color: '#848E9C' }}>
              Quick Close
            </label>
            <div className="grid grid-cols-4 gap-2">
              {[25, 50, 75, 100].map((pct) => (
                <button
                  key={pct}
                  type="button"
                  onClick={() => handleQuickClose(pct)}
                  className="px-2 py-1 rounded text-sm transition-all hover:scale-105 active:scale-95"
                  style={quickButtonStyle}
                >
                  {pct}%
                </button>
              ))}
            </div>
          </div>

          {/* Display calculated close amount */}
          {quantity && !isNaN(parseFloat(quantity)) && (
            <div className="p-3 rounded text-sm" style={{ background: 'rgba(255, 255, 255, 0.05)', border: '1px solid #2B3139', color: '#EAECEF' }}>
              Close: <span className="font-bold" style={{ color: '#F0B90B' }}>{parseFloat(quantity).toFixed(4)}</span>
              {' '}(Remaining: <span style={{ color: '#0ECB81' }}>
                {(currentQuantity - parseFloat(quantity)).toFixed(4)}
              </span>)
            </div>
          )}

          {/* Buttons */}
          <div className="flex gap-3 pt-4">
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 px-4 py-2 rounded font-medium transition-all hover:scale-105 active:scale-95 disabled:opacity-50"
              style={{
                ...dangerButtonStyle,
                opacity: isSubmitting ? 0.5 : 1,
              }}
            >
              {isSubmitting ? 'Closing...' : 'Close Position'}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded font-medium transition-all hover:scale-105 active:scale-95"
              style={quickButtonStyle}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default PartialCloseModal;
