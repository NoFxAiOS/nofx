import React, { useState } from 'react';
import { X } from 'lucide-react';
import { httpClient } from '../lib/httpClient';

interface PlaceOrderModalProps {
  traderID: string;
  onClose: () => void;
  onSuccess: () => void;
}

const PlaceOrderModal: React.FC<PlaceOrderModalProps> = ({
  traderID,
  onClose,
  onSuccess,
}) => {
  const [symbol, setSymbol] = useState('BTCUSDT');
  const [side, setSide] = useState('buy');
  const [orderType, setOrderType] = useState('limit');
  const [quantity, setQuantity] = useState('');
  const [price, setPrice] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  const panelStyle = {
    background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
    border: '1px solid #2B3139',
    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.2)',
  }
  const buttonStyle = {
    background: 'linear-gradient(135deg, #F0B90B 0%, #D4A506 100%)',
    color: '#1E2329',
    fontWeight: '600',
  }
  const inputStyle = {
    background: 'rgba(255, 255, 255, 0.05)',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!symbol || !quantity) {
      setError('Symbol and quantity are required');
      return;
    }

    if (orderType === 'limit' && !price) {
      setError('Price is required for limit orders');
      return;
    }

    setIsSubmitting(true);

    try {
      const result = await httpClient.post<{ error?: string }>(
        `/api/pending-orders/place?trader_id=${traderID}`,
        {
          symbol,
          side,
          quantity: parseFloat(quantity),
          price: orderType === 'limit' ? parseFloat(price) : 0,
          order_type: orderType,
        }
      );

      if (!result.success) {
        setError(result.message || 'Failed to place order');
        return;
      }

      alert('Order placed successfully');
      onSuccess();
    } catch (err) {
      setError(`Error: ${err}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="rounded-lg p-6 max-w-md w-full" style={panelStyle}>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>Place Order</h2>
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
          {/* Symbol */}
          <div>
            <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
              Symbol
            </label>
            <input
              type="text"
              value={symbol}
              onChange={(e) => setSymbol(e.target.value.toUpperCase())}
              placeholder="e.g., BTCUSDT"
              className="w-full px-3 py-2 rounded transition-all"
              style={inputStyle}
            />
          </div>

          {/* Side */}
          <div>
            <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
              Side
            </label>
            <div className="flex gap-2">
              <label className="flex-1 flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="side"
                  value="buy"
                  checked={side === 'buy'}
                  onChange={(e) => setSide(e.target.value)}
                  className="cursor-pointer"
                />
                <span style={{ color: '#EAECEF' }}>Buy</span>
              </label>
              <label className="flex-1 flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="side"
                  value="sell"
                  checked={side === 'sell'}
                  onChange={(e) => setSide(e.target.value)}
                  className="cursor-pointer"
                />
                <span style={{ color: '#EAECEF' }}>Sell</span>
              </label>
            </div>
          </div>

          {/* Order Type */}
          <div>
            <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
              Order Type
            </label>
            <div className="flex gap-2">
              <label className="flex-1 flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="orderType"
                  value="limit"
                  checked={orderType === 'limit'}
                  onChange={(e) => setOrderType(e.target.value)}
                  className="cursor-pointer"
                />
                <span style={{ color: '#EAECEF' }}>Limit</span>
              </label>
              <label className="flex-1 flex items-center gap-2 cursor-pointer">
                <input
                  type="radio"
                  name="orderType"
                  value="market"
                  checked={orderType === 'market'}
                  onChange={(e) => setOrderType(e.target.value)}
                  className="cursor-pointer"
                />
                <span style={{ color: '#EAECEF' }}>Market</span>
              </label>
            </div>
          </div>

          {/* Quantity */}
          <div>
            <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
              Quantity
            </label>
            <input
              type="number"
              step="0.0001"
              min="0"
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              placeholder="0.0000"
              className="w-full px-3 py-2 rounded transition-all"
              style={inputStyle}
            />
          </div>

          {/* Price (for limit orders) */}
          {orderType === 'limit' && (
            <div>
              <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
                Price
              </label>
              <input
                type="number"
                step="0.01"
                min="0"
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                placeholder="0.00"
                className="w-full px-3 py-2 rounded transition-all"
                style={inputStyle}
              />
            </div>
          )}

          {/* Buttons */}
          <div className="flex gap-3 pt-4">
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 px-4 py-2 rounded font-medium transition-all hover:scale-105 active:scale-95 disabled:opacity-50"
              style={{
                ...buttonStyle,
                opacity: isSubmitting ? 0.5 : 1,
              }}
            >
              {isSubmitting ? 'Placing...' : 'Place Order'}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded font-medium transition-all hover:scale-105 active:scale-95"
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                color: '#EAECEF',
                border: '1px solid #2B3139',
              }}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default PlaceOrderModal;
