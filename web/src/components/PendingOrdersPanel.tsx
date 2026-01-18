import React, { useState } from 'react';
import { Edit2, Trash2, Plus } from 'lucide-react';
import PlaceOrderModal from './PlaceOrderModal';

export interface PendingOrder {
  orderId: string;
  symbol: string;
  side: string;
  type: string;
  price: number;
  stopPrice: number;
  quantity: number;
  status: string;
  createdTime?: number;
}

interface PendingOrdersPanelProps {
  traderID: string;
  orders: PendingOrder[];
  onRefresh: () => void;
  onModify?: (order: PendingOrder) => void;
  onCancel?: (orderID: string, symbol: string) => void;
}

export const PendingOrdersPanel: React.FC<PendingOrdersPanelProps> = ({
  traderID,
  orders,
  onRefresh,
  onModify,
  onCancel,
}: PendingOrdersPanelProps) => {
  const [selectedOrder, setSelectedOrder] = useState<PendingOrder | null>(null);
  const [showPlaceOrderModal, setShowPlaceOrderModal] = useState(false);
  const [isModifying, setIsModifying] = useState(false);

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

  const handleCancelOrder = async (orderId: string, symbol: string) => {
    if (!window.confirm('Are you sure you want to cancel this order?')) {
      return;
    }

    try {
      const response = await fetch(
        `/api/pending-orders/${orderId}?trader_id=${traderID}`,
        {
          method: 'DELETE',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ symbol }),
        }
      );

      if (!response.ok) {
        const error = await response.json();
        alert(`Failed to cancel order: ${error.error}`);
        return;
      }

      alert('Order canceled successfully');
      onCancel?.(orderId, symbol);
      onRefresh();
    } catch (error) {
      alert(`Error canceling order: ${error}`);
    }
  };

  const handleModifyOrder = (order: PendingOrder) => {
    setSelectedOrder(order);
    setIsModifying(true);
  };

  const handleModifySubmit = async (newQuantity: number, newPrice: number) => {
    if (!selectedOrder) return;

    try {
      const response = await fetch(
        `/api/pending-orders/${selectedOrder.orderId}/modify?trader_id=${traderID}`,
        {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            symbol: selectedOrder.symbol,
            quantity: newQuantity || selectedOrder.quantity,
            price: newPrice || selectedOrder.price,
          }),
        }
      );

      if (!response.ok) {
        const error = await response.json();
        alert(`Failed to modify order: ${error.error}`);
        return;
      }

      alert('Order modified successfully');
      setIsModifying(false);
      setSelectedOrder(null);
      onModify?.(selectedOrder);
      onRefresh();
    } catch (error) {
      alert(`Error modifying order: ${error}`);
    }
  };

  return (
    <div className="rounded-lg p-6 transition-all duration-200" style={panelStyle}>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-bold" style={{ color: '#EAECEF' }}>Pending Orders</h3>
        <button
          onClick={() => setShowPlaceOrderModal(true)}
          className="flex items-center gap-2 px-3 py-2 rounded text-sm transition-all hover:scale-105 active:scale-95"
          style={buttonStyle}
        >
          <Plus size={16} />
          Place Order
        </button>
      </div>

      {orders.length === 0 ? (
        <div className="text-center py-8" style={{ color: '#848E9C' }}>
          No pending orders
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr style={{ borderBottom: '1px solid #2B3139' }}>
                <th className="text-left py-2 px-3" style={{ color: '#848E9C' }}>Symbol</th>
                <th className="text-left py-2 px-3" style={{ color: '#848E9C' }}>Side</th>
                <th className="text-left py-2 px-3" style={{ color: '#848E9C' }}>Type</th>
                <th className="text-right py-2 px-3" style={{ color: '#848E9C' }}>Quantity</th>
                <th className="text-right py-2 px-3" style={{ color: '#848E9C' }}>Price</th>
                <th className="text-center py-2 px-3" style={{ color: '#848E9C' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr
                  key={order.orderId}
                  className="transition-all duration-200 hover:bg-white/5"
                  style={{ borderBottom: '1px solid #2B3139' }}
                >
                  <td className="py-3 px-3 font-medium" style={{ color: '#EAECEF' }}>{order.symbol}</td>
                  <td className={`py-3 px-3 font-medium`} style={{ color: order.side.toLowerCase() === 'buy' ? '#0ECB81' : '#F6465D' }}>
                    {order.side.toUpperCase()}
                  </td>
                  <td className="py-3 px-3" style={{ color: '#848E9C' }}>{order.type}</td>
                  <td className="py-3 px-3 text-right" style={{ color: '#848E9C' }}>{order.quantity.toFixed(4)}</td>
                  <td className="py-3 px-3 text-right" style={{ color: '#848E9C' }}>
                    {order.price > 0 ? order.price.toFixed(4) : '-'}
                  </td>
                  <td className="py-3 px-3">
                    <div className="flex items-center justify-center gap-2">
                      <button
                        onClick={() => handleModifyOrder(order)}
                        className="p-1 rounded transition-all hover:scale-110 active:scale-95"
                        style={{ color: '#F0B90B', background: 'rgba(240, 185, 11, 0.1)' }}
                        title="Modify order"
                      >
                        <Edit2 size={16} />
                      </button>
                      <button
                        onClick={() => handleCancelOrder(order.orderId, order.symbol)}
                        className="p-1 rounded transition-all hover:scale-110 active:scale-95"
                        style={{ color: '#F6465D', background: 'rgba(246, 70, 93, 0.1)' }}
                        title="Cancel order"
                      >
                        <Trash2 size={16} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Modify Order Modal */}
      {isModifying && selectedOrder && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="rounded-lg p-6 max-w-md w-full" style={panelStyle}>
            <h2 className="text-xl font-bold mb-4" style={{ color: '#EAECEF' }}>Modify Order</h2>
            <ModifyOrderForm
              order={selectedOrder}
              onSubmit={handleModifySubmit}
              onCancel={() => {
                setIsModifying(false);
                setSelectedOrder(null);
              }}
            />
          </div>
        </div>
      )}

      {/* Place Order Modal */}
      {showPlaceOrderModal && (
        <PlaceOrderModal
          traderID={traderID}
          onClose={() => setShowPlaceOrderModal(false)}
          onSuccess={() => {
            setShowPlaceOrderModal(false);
            onRefresh();
          }}
        />
      )}
    </div>
  );
};

interface ModifyOrderFormProps {
  order: PendingOrder;
  onSubmit: (quantity: number, price: number) => void;
  onCancel: () => void;
}

const ModifyOrderForm: React.FC<ModifyOrderFormProps> = ({
  order,
  onSubmit,
  onCancel,
}) => {
  const [quantity, setQuantity] = useState(order.quantity);
  const [price, setPrice] = useState(order.price);
  const [isSubmitting, setIsSubmitting] = useState(false);

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
    setIsSubmitting(true);
    try {
      onSubmit(quantity, price);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
          Quantity
        </label>
        <input
          type="number"
          step="0.0001"
          min="0"
          value={quantity}
          onChange={(e) => setQuantity(parseFloat(e.target.value) || 0)}
          className="w-full px-3 py-2 rounded transition-all"
          style={inputStyle}
        />
      </div>

      <div>
        <label className="block text-sm font-medium mb-1" style={{ color: '#848E9C' }}>
          Price
        </label>
        <input
          type="number"
          step="0.01"
          min="0"
          value={price}
          onChange={(e) => setPrice(parseFloat(e.target.value) || 0)}
          className="w-full px-3 py-2 rounded transition-all"
          style={inputStyle}
        />
      </div>

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
          {isSubmitting ? 'Updating...' : 'Update'}
        </button>
        <button
          type="button"
          onClick={onCancel}
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
  );
};

export default PendingOrdersPanel;
