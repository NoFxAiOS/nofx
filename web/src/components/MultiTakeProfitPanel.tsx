import React, { useState } from 'react';
import { Plus, Trash2, Edit2 } from 'lucide-react';

interface TakeProfitTier {
  level: number;
  takeProfitPrice: number;
  quantity: number;
  status: string;
}

interface MultiTakeProfitPanelProps {
  traderID: string;
  symbol: string;
  positionSide: string;
  currentQuantity: number;
  tiers: TakeProfitTier[];
  onUpdate: () => void;
  onRefresh?: () => void;
}

const MultiTakeProfitPanel: React.FC<MultiTakeProfitPanelProps> = ({
  traderID,
  symbol,
  positionSide,
  currentQuantity,
  tiers,
  onUpdate,
}) => {
  const [showAddForm, setShowAddForm] = useState(false);
  const [editingTier, setEditingTier] = useState<number | null>(null);
  const [takeProfitPrices, setTakeProfitPrices] = useState<string[]>(['', '']);
  const [isSubmitting, setIsSubmitting] = useState(false);

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

  const handleAddTiers = async () => {
    const prices = takeProfitPrices
      .map((p) => parseFloat(p))
      .filter((p) => !isNaN(p) && p > 0);

    if (prices.length === 0) {
      alert('Please enter at least one take profit price');
      return;
    }

    setIsSubmitting(true);

    try {
      const response = await fetch(
        `/api/stop-orders/set-multiple-tp?trader_id=${traderID}`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            symbol,
            position_side: positionSide,
            quantity: currentQuantity,
            take_profit_prices: prices,
          }),
        }
      );

      if (!response.ok) {
        const error = await response.json();
        alert(`Failed to set take profits: ${error.error}`);
        return;
      }

      alert('Take profit orders set successfully');
      setTakeProfitPrices(['', '']);
      setShowAddForm(false);
      onUpdate();
    } catch (error) {
      alert(`Error: ${error}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleModifyTier = async (tier: number, newPrice: number) => {
    if (newPrice <= 0) {
      alert('Price must be greater than 0');
      return;
    }

    setIsSubmitting(true);

    try {
      const response = await fetch(
        `/api/stop-orders/tp-tier/${tier}?trader_id=${traderID}`,
        {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            symbol,
            take_profit_price: newPrice,
          }),
        }
      );

      if (!response.ok) {
        const error = await response.json();
        alert(`Failed to modify tier: ${error.error}`);
        return;
      }

      alert('Tier modified successfully');
      setEditingTier(null);
      onUpdate();
    } catch (error) {
      alert(`Error: ${error}`);
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteTier = async (_tier: number) => {
    if (!window.confirm('Delete this take profit tier?')) return;

    // Note: Bitget doesn't support deleting individual tiers
    // You would need to cancel all and re-add the remaining ones
    alert('To delete a tier, cancel all take profits and set new ones');
  };

  return (
    <div className="rounded-lg p-6 transition-all duration-200" style={panelStyle}>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-bold" style={{ color: '#EAECEF' }}>Take Profit Orders</h3>
        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="flex items-center gap-2 px-3 py-2 rounded text-sm transition-all hover:scale-105 active:scale-95"
          style={buttonStyle}
        >
          <Plus size={16} />
          Add Tiers
        </button>
      </div>

      {showAddForm && (
        <div className="mb-4 p-4 rounded" style={{ background: 'rgba(255, 255, 255, 0.05)', border: '1px solid #2B3139' }}>
          <h4 className="text-sm font-semibold mb-3" style={{ color: '#EAECEF' }}>Add Take Profit Tiers</h4>
          <div className="space-y-2">
            {takeProfitPrices.map((price, idx) => (
              <div key={idx} className="flex gap-2">
                <input
                  type="number"
                  step="0.01"
                  placeholder={`Take Profit Price ${idx + 1}`}
                  value={price}
                  onChange={(e) => {
                    const newPrices = [...takeProfitPrices];
                    newPrices[idx] = e.target.value;
                    setTakeProfitPrices(newPrices);
                  }}
                  className="flex-1 px-3 py-2 rounded transition-all"
                  style={inputStyle}
                />
              </div>
            ))}
            <button
              onClick={() => setTakeProfitPrices([...takeProfitPrices, ''])}
              className="text-sm transition-all hover:scale-105"
              style={{ color: '#F0B90B' }}
            >
              + Add another tier
            </button>
          </div>
          <div className="flex gap-2 mt-4">
            <button
              onClick={handleAddTiers}
              disabled={isSubmitting}
              className="flex-1 px-3 py-2 rounded text-sm transition-all hover:scale-105 active:scale-95 disabled:opacity-50"
              style={{
                ...buttonStyle,
                opacity: isSubmitting ? 0.5 : 1,
              }}
            >
              {isSubmitting ? 'Setting...' : 'Set Tiers'}
            </button>
            <button
              onClick={() => setShowAddForm(false)}
              className="flex-1 px-3 py-2 rounded text-sm transition-all hover:scale-105 active:scale-95"
              style={{
                background: 'rgba(255, 255, 255, 0.05)',
                color: '#EAECEF',
                border: '1px solid #2B3139',
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {tiers.length === 0 ? (
        <div className="text-center py-8" style={{ color: '#848E9C' }}>
          No take profit orders set
        </div>
      ) : (
        <div className="space-y-2">
          {tiers.map((tier) => (
            <TierRow
              key={tier.level}
              tier={tier}
              onModify={handleModifyTier}
              onDelete={handleDeleteTier}
              isEditing={editingTier === tier.level}
              onEditChange={(edit) => setEditingTier(edit ? tier.level : null)}
            />
          ))}
        </div>
      )}
    </div>
  );
};

interface TierRowProps {
  tier: TakeProfitTier;
  onModify: (tier: number, newPrice: number) => void;
  onDelete: (tier: number) => void;
  isEditing: boolean;
  onEditChange: (editing: boolean) => void;
}

const TierRow: React.FC<TierRowProps> = ({
  tier,
  onModify,
  onDelete,
  isEditing,
  onEditChange,
}) => {
  const [newPrice, setNewPrice] = useState(tier.takeProfitPrice.toString());

  const panelStyle = {
    background: 'linear-gradient(135deg, #1E2329 0%, #181C21 100%)',
    border: '1px solid #2B3139',
  }
  const inputStyle = {
    background: 'rgba(255, 255, 255, 0.05)',
    border: '1px solid #2B3139',
    color: '#EAECEF',
  }
  const buttonStyle = {
    background: 'linear-gradient(135deg, #F0B90B 0%, #D4A506 100%)',
    color: '#1E2329',
    fontWeight: '600',
  }

  if (isEditing) {
    return (
      <div className="flex gap-2 p-3 rounded items-center" style={panelStyle}>
        <span className="text-sm" style={{ color: '#848E9C' }}>Tier {tier.level}:</span>
        <input
          type="number"
          step="0.01"
          value={newPrice}
          onChange={(e) => setNewPrice(e.target.value)}
          className="flex-1 px-2 py-1 rounded text-sm transition-all"
          style={inputStyle}
        />
        <button
          onClick={() => {
            onModify(tier.level, parseFloat(newPrice));
            onEditChange(false);
          }}
          className="px-2 py-1 rounded text-sm transition-all hover:scale-105 active:scale-95"
          style={buttonStyle}
        >
          Save
        </button>
        <button
          onClick={() => onEditChange(false)}
          className="px-2 py-1 rounded text-sm transition-all hover:scale-105 active:scale-95"
          style={{
            background: 'rgba(255, 255, 255, 0.05)',
            color: '#EAECEF',
            border: '1px solid #2B3139',
          }}
        >
          Cancel
        </button>
      </div>
    );
  }

  return (
    <div className="flex gap-2 p-3 rounded items-center transition-all duration-200 hover:bg-white/5" style={{ ...panelStyle, borderBottom: '1px solid #2B3139' }}>
      <div className="flex-1">
        <div className="text-sm" style={{ color: '#EAECEF' }}>
          Tier {tier.level} @ {tier.takeProfitPrice.toFixed(2)}
        </div>
        <div className="text-xs" style={{ color: '#848E9C' }}>
          Qty: {tier.quantity.toFixed(4)}
        </div>
      </div>
      <div className="flex gap-1">
        <button
          onClick={() => onEditChange(true)}
          className="p-1 rounded transition-all hover:scale-110 active:scale-95"
          style={{ color: '#F0B90B', background: 'rgba(240, 185, 11, 0.1)' }}
          title="Edit tier"
        >
          <Edit2 size={14} />
        </button>
        <button
          onClick={() => onDelete(tier.level)}
          className="p-1 rounded transition-all hover:scale-110 active:scale-95"
          style={{ color: '#F6465D', background: 'rgba(246, 70, 93, 0.1)' }}
          title="Delete tier"
        >
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  );
};

export default MultiTakeProfitPanel;
