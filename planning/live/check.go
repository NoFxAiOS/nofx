package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"nofx/planning"
	"nofx/planning/core"
)

// FinishManualClose cleans only this plan's verified exit orders after an
// explicit, confirmed manual close. It cannot submit orders or restart entries.
func (c *Controller) FinishManualClose(ctx context.Context, user, id string) (View, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	lock := c.runLock(id)
	if err := lock.LockContext(ctx); err != nil {
		return View{}, err
	}
	err := func() error {
		v, err := c.Get(user, id)
		if err != nil {
			return err
		}
		if v.State == "completed" || v.State == "cancelled" {
			return nil
		}
		if v.State == "approved" || v.State == "starting" || v.State == "running" {
			return errors.New("Stop remaining entries before closing")
		}
		broker, key, err := c.Factory(ctx, user, v.ExchangeID)
		if err != nil || key != v.AccountKey {
			return errors.New("Could not verify the closed account")
		}
		account, err := broker.Account(ctx, v.Plan.Snapshot.Symbol)
		if _, parseErr := decimal(account.Position); err != nil || parseErr != nil || cmp(account.Position, "0") != 0 {
			return errors.New("Position is not confirmed flat; exit orders were kept")
		}
		for _, order := range v.Orders {
			if order.Role == "entry" && order.Status != "FILLED" && order.Status != "CANCELED" && order.Status != "REJECTED" && order.Status != "EXPIRED" {
				return errors.New("Remaining entries must be reconciled before removing exits")
			}
		}
		for _, order := range v.Orders {
			if order.Role != "sl" && order.Role != "tp" {
				continue
			}
			observed, err := broker.Lookup(ctx, order)
			if err != nil || !validReceipt(order, observed) || (order.OrderID != "" && order.OrderID != observed.OrderID) {
				return errors.New("Exit order is unconfirmed; inspect exchange orders")
			}
			if observed.Status == "NEW" || observed.Status == "PARTIALLY_FILLED" {
				if err := broker.Cancel(ctx, observed); err != nil {
					return errors.New("Exit cancellation is unconfirmed; inspect exchange orders")
				}
			}
		}
		return nil
	}()
	lock.Unlock()
	if err != nil {
		return View{}, err
	}
	return c.CheckStatus(ctx, user, id)
}

// CheckStatus only reads the exchange. It never starts a worker, submits an
// order, or cancels an exchange order. A flat, fully resolved basket can release
// its account reservation, including after an externally closed position.
func (c *Controller) CheckStatus(ctx context.Context, user, id string) (View, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	lock := c.runLock(id)
	if err := lock.LockContext(ctx); err != nil {
		return View{}, err
	}
	defer lock.Unlock()
	v, err := c.Get(user, id)
	if err != nil {
		return v, err
	}
	if v.State == "approved" && !time.Now().Before(v.Plan.Expires) {
		err = c.retireUnusedApproval(&v, "expired", "Unused approval expired; no order was submitted.")
		return v, err
	}
	if v.State != "needs_attention" && v.State != "position_open" && v.State != "stopping" {
		return v, nil
	}
	broker, key, err := c.Factory(ctx, user, v.ExchangeID)
	if err != nil || key != v.AccountKey || broker.Venue() != v.Plan.Snapshot.Venue {
		return v, errors.New("Could not verify the original trading account. Check its connection in Config.")
	}
	for i, order := range v.Orders {
		if order.Status == "scheduled" || (order.Role == "entry" && (order.Status == "CANCELED" || order.Status == "EXPIRED") && order.OrderID == "" && cmp(order.Filled, "0") == 0) {
			continue // Persisted intent has never been sent to the exchange.
		}
		observed, lookupErr := broker.Lookup(ctx, order)
		if lookupErr != nil || (order.OrderID != "" && order.OrderID != observed.OrderID) || !validReceipt(order, observed) || cmp(observed.Filled, order.Filled) < 0 {
			v.State = "needs_attention"
			v.Error = fmt.Sprintf("Could not confirm %s order %s. Check Positions & orders; new plans can still be generated, but another entry cannot be submitted yet.", orderRoleName(order.Role), order.OrderID)
			if err := c.save(&v); err != nil {
				return v, err
			}
			return v, errors.New(v.Error)
		}
		v.Orders[i] = observed
	}
	account, err := broker.Account(ctx, v.Plan.Snapshot.Symbol)
	if err != nil {
		return v, errors.New("Could not read live positions and open orders. The account remains reserved. Try checking again.")
	}
	_, positionErr := decimal(account.Position)
	_, equityErr := decimal(account.Equity)
	_, availableErr := decimal(account.Available)
	if positionErr != nil || equityErr != nil || availableErr != nil || account.OpenOrders < 0 {
		return v, errors.New("The exchange returned incomplete position or order data. The account remains reserved.")
	}
	if confirmedProtectedPosition(v.Orders, account) {
		// Reconciliation never resumes a paused TWAP or releases another entry.
		// Scheduled rows are local intent, so withdrawing them has no exchange effect.
		for i := range v.Orders {
			if v.Orders[i].Status == "scheduled" {
				v.Orders[i].Status = "CANCELED"
			}
		}
		v.State, v.Error = "position_open", ""
		return v, c.save(&v)
	}
	if !confirmedClosedTrade(v.Orders, account) {
		v.State = "needs_attention"
		v.Error = "This trade still has a position or order that could not be fully reconciled. Check Positions & orders before submitting another entry. You can still generate new plans."
		if err = c.save(&v); err != nil {
			return v, err
		}
		return v, errors.New(v.Error)
	}
	err = c.complete(&v)
	return v, err
}

func (c *Controller) complete(v *View) error {
	for i := range v.Orders {
		if v.Orders[i].Status == "scheduled" {
			v.Orders[i].Status = "CANCELED"
		}
	}
	v.State, v.Error, v.UpdatedAt = "completed", "", time.Now().UTC()
	raw, err := json.Marshal(v.Orders)
	if err != nil {
		return err
	}
	return c.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Run{}).Where("id = ? AND user_id = ?", v.ID, v.UserID).Updates(map[string]any{"state": v.State, "orders_json": string(raw), "error": "", "updated_at": v.UpdatedAt}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id = ? AND user_id = ?", v.ID, v.UserID).Delete(&Lease{}).Error; err != nil {
			return err
		}
		return tx.Model(&planning.Draft{}).Where("user_id = ? AND id = ?", v.UserID, v.PlanID).Update("status", "completed").Error
	})
}

// confirmedClosedTrade is shared by background monitoring and manual checks.
// Flatness alone is insufficient: a newly filled entry can precede its position
// snapshot, and an unresolved stop can still execute after an external close.
func confirmedClosedTrade(orders []Order, account Account) bool {
	if cmp(account.Position, "0") != 0 || account.OpenOrders != 0 {
		return false
	}
	for _, order := range orders {
		switch order.Status {
		case "scheduled", "FILLED", "CANCELED", "REJECTED", "EXPIRED":
		default:
			return false
		}
	}
	remaining := cmp(filledExposure(orders), "0")
	if remaining < 0 {
		return false
	}
	if remaining == 0 {
		return true
	}
	for _, entry := range orders {
		if entry.Role != "entry" || cmp(entry.Filled, "0") <= 0 {
			continue
		}
		stop, target := core.Decimal("0"), core.Decimal("0")
		for _, exit := range orders {
			if exit.ParentID != entry.ClientID || (exit.Status != "CANCELED" && exit.Status != "FILLED" && exit.Status != "EXPIRED") {
				continue
			}
			if exit.Role == "sl" {
				stop = add(stop, exit.Quantity)
			}
			if exit.Role == "tp" {
				target = add(target, exit.Quantity)
			}
		}
		if cmp(stop, entry.Filled) < 0 || cmp(target, entry.Filled) < 0 {
			return false
		}
	}
	return true
}

func orderRoleName(role string) string {
	switch role {
	case "sl":
		return "stop-loss"
	case "tp":
		return "take-profit"
	default:
		return "entry"
	}
}

// confirmedProtectedPosition requires complete, verified receipts before a
// transient read failure can be cleared. It does not authorize new entries.
func confirmedProtectedPosition(orders []Order, account Account) bool {
	if cmp(account.Position, "0") <= 0 || cmp(account.Position, filledExposure(orders)) != 0 {
		return false
	}
	for _, order := range orders {
		if order.Status == "scheduled" && order.Role == "entry" && cmp(order.Filled, "0") == 0 {
			continue
		}
		if order.Role == "entry" {
			switch order.Status {
			case "FILLED", "CANCELED", "REJECTED", "EXPIRED":
			default:
				return false
			}
		} else {
			switch order.Status {
			case "NEW", "FILLED", "CANCELED", "REJECTED", "EXPIRED":
			default:
				return false
			}
		}
	}
	for _, entry := range orders {
		if entry.Role != "entry" || cmp(entry.Filled, "0") <= 0 {
			continue
		}
		closed, stop, target := core.Decimal("0"), core.Decimal("0"), core.Decimal("0")
		for _, exit := range orders {
			if exit.ParentID != entry.ClientID {
				continue
			}
			closed = add(closed, exit.Filled)
			if exit.Status != "NEW" {
				continue
			}
			if exit.Role == "sl" {
				stop = add(stop, exit.Quantity)
			}
			if exit.Role == "tp" {
				target = add(target, exit.Quantity)
			}
		}
		remaining := sub(entry.Filled, closed)
		if cmp(remaining, "0") < 0 || cmp(stop, remaining) != 0 || cmp(target, remaining) != 0 {
			return false
		}
	}
	return true
}
