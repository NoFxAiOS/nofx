package store

import (
	"fmt"
	"math"
	"nofx/logger"
	"strings"
	"time"
)

// PositionBuilder handles position creation and updates with support for:
// - Position averaging (merging multiple opens)
// - Partial closes (reducing quantity)
// - FIFO matching
// - Time-ordered processing
type PositionBuilder struct {
	positionStore *PositionStore
}

// NewPositionBuilder creates a new PositionBuilder
func NewPositionBuilder(positionStore *PositionStore) *PositionBuilder {
	return &PositionBuilder{
		positionStore: positionStore,
	}
}

// ProcessTrade processes a single trade and updates position accordingly
func (pb *PositionBuilder) ProcessTrade(
	traderID, exchangeID, exchangeType, symbol, side, action string,
	quantity, price, fee, realizedPnL float64,
	tradeTime time.Time,
	orderID string,
) error {
	if strings.HasPrefix(action, "open_") {
		return pb.handleOpen(traderID, exchangeID, exchangeType, symbol, side, quantity, price, fee, tradeTime, orderID)
	} else if strings.HasPrefix(action, "close_") {
		return pb.handleClose(traderID, symbol, side, quantity, price, fee, realizedPnL, tradeTime, orderID)
	}
	return nil
}

// handleOpen handles opening positions (create new or average into existing)
func (pb *PositionBuilder) handleOpen(
	traderID, exchangeID, exchangeType, symbol, side string,
	quantity, price, fee float64,
	tradeTime time.Time,
	orderID string,
) error {
	// Get existing OPEN position for (symbol, side)
	existing, err := pb.positionStore.GetOpenPositionBySymbol(traderID, symbol, side)
	if err != nil {
		return fmt.Errorf("failed to get open position: %w", err)
	}

	if existing == nil {
		// Create new position
		position := &TraderPosition{
			TraderID:           traderID,
			ExchangeID:         exchangeID,
			ExchangeType:       exchangeType,
			ExchangePositionID: fmt.Sprintf("sync_%s_%s_%d", symbol, side, tradeTime.UnixMilli()),
			Symbol:             symbol,
			Side:               side,
			Quantity:           quantity,
			EntryPrice:         price,
			EntryOrderID:       orderID,
			EntryTime:          tradeTime,
			Leverage:           1,
			Status:             "OPEN",
			Source:             "sync",
			Fee:                fee,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}
		return pb.positionStore.CreateOpenPosition(position)
	}

	// Merge: Calculate weighted average entry price and update position
	logger.Infof("  📊 Averaging position: %s %s %.6f @ %.2f + %.6f @ %.2f",
		symbol, side, existing.Quantity, existing.EntryPrice, quantity, price)

	return pb.positionStore.UpdatePositionQuantityAndPrice(existing.ID, quantity, price, fee)
}

// handleClose handles closing positions (partial or full)
func (pb *PositionBuilder) handleClose(
	traderID, symbol, side string,
	quantity, price, fee, realizedPnL float64,
	tradeTime time.Time,
	orderID string,
) error {
	// Get OPEN position
	position, err := pb.positionStore.GetOpenPositionBySymbol(traderID, symbol, side)
	if err != nil {
		return fmt.Errorf("failed to get open position: %w", err)
	}

	if position == nil {
		// No open position, log warning and skip
		logger.Infof("  ⚠️  No matching open position for %s %s (orderID: %s), skipping", symbol, side, orderID)
		return nil
	}

	const QUANTITY_TOLERANCE = 0.0001

	if quantity < position.Quantity-QUANTITY_TOLERANCE {
		// Partial close: reduce quantity
		logger.Infof("  📉 Partial close: %s %s %.6f → %.6f (closed %.6f @ %.2f)",
			symbol, side, position.Quantity, position.Quantity-quantity, quantity, price)
		return pb.positionStore.ReducePositionQuantity(position.ID, quantity, fee)
	} else {
		// Full close (or close with tolerance): mark as CLOSED
		closeQty := quantity
		if quantity > position.Quantity {
			logger.Infof("  ⚠️  Over-close detected: %s %s trying to close %.6f but only %.6f open, closing full position",
				symbol, side, quantity, position.Quantity)
			closeQty = position.Quantity
		}

		logger.Infof("  ✅ Full close: %s %s %.6f @ %.2f (entry: %.2f, PnL: %.2f)",
			symbol, side, closeQty, price, position.EntryPrice, realizedPnL)

		// Calculate total fee (existing + new)
		totalFee := position.Fee + fee

		return pb.positionStore.ClosePositionFully(
			position.ID,
			price,
			orderID,
			tradeTime,
			realizedPnL,
			totalFee,
			"sync",
		)
	}
}

// quantitiesMatch checks if two quantities are close enough (within tolerance)
func quantitiesMatch(a, b float64) bool {
	const QUANTITY_TOLERANCE = 0.0001
	return math.Abs(a-b) < QUANTITY_TOLERANCE
}
