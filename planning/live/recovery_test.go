package live

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func protectedTrade(t *testing.T, kind string) (*Controller, *testBroker, View) {
	t.Helper()
	c, b, _, p := setup(t, kind)
	v := started(t, c, p)
	c.step(context.Background(), "alice", v.ID)
	v, _ = c.Get("alice", v.ID)
	entry := b.orders[v.Orders[0].ClientID]
	entry.Status, entry.Filled = "FILLED", entry.Quantity
	b.orders[entry.ClientID] = entry
	b.account.Position = entry.Quantity
	c.step(context.Background(), "alice", v.ID)
	v, _ = c.Get("alice", v.ID)
	v.State, v.Error = "needs_attention", "Previous position snapshot was not yet available"
	if err := c.save(&v); err != nil {
		t.Fatal(err)
	}
	return c, b, v
}

func TestStatusCheckRecoversOnlyVerifiedProtectedPosition(t *testing.T) {
	for _, scenario := range []string{"protected", "missing_stop", "triggered_stop", "unknown", "changed_order_id", "position_mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			c, b, v := protectedTrade(t, "limit")
			stop := b.orders[v.Orders[1].ClientID]
			switch scenario {
			case "missing_stop":
				stop.Status = "CANCELED"
			case "triggered_stop":
				stop.Status = "TRIGGERED"
			case "unknown":
				b.lookupErr = true
			case "changed_order_id":
				stop.OrderID = "unrelated-order"
			case "position_mismatch":
				b.account.Position = add(b.account.Position, "0.01")
			}
			b.orders[stop.ClientID] = stop
			placed, cancelled := len(b.placed), len(b.cancelled)
			out, err := c.CheckStatus(context.Background(), "alice", v.ID)
			if scenario == "protected" {
				if err != nil || out.State != "position_open" || out.Error != "" {
					t.Fatalf("verified position stayed interrupted: %s %s %v", out.State, out.Error, err)
				}
			} else if err == nil || out.State != "needs_attention" {
				t.Fatalf("unverified trade marked healthy: %s %v", out.State, err)
			}
			var leases int64
			c.DB.Model(&Lease{}).Count(&leases)
			if leases != 1 || len(b.placed) != placed || len(b.cancelled) != cancelled || len(c.workers) != 0 {
				t.Fatal("read-only reconciliation released account, changed exchange orders or restarted entries")
			}
		})
	}
}

func TestStatusRecoveryDoesNotResumeRemainingTWAPSlices(t *testing.T) {
	c, b, v := protectedTrade(t, "twap")
	if v.Orders[1].Status != "scheduled" {
		t.Fatal("expected an unreleased second TWAP slice")
	}
	placed, cancelled := len(b.placed), len(b.cancelled)
	for range 2 {
		out, err := c.CheckStatus(context.Background(), "alice", v.ID)
		if err != nil || out.State != "position_open" || out.Orders[1].Status != "CANCELED" || out.Orders[1].OrderID != "" {
			t.Fatalf("paused TWAP was not reconciled without replay: %+v %v", out, err)
		}
	}
	if len(b.placed) != placed || len(b.cancelled) != cancelled || len(c.workers) != 0 {
		t.Fatal("status check submitted or cancelled exchange orders")
	}
}

func TestStatusCheckReconcilesLocallyExpiredTWAPSlice(t *testing.T) {
	for _, scenario := range []string{"protected", "restarted_protected", "restarted_closed", "restarted_open_stop", "restarted_unknown_stop", "restarted_open_order"} {
		t.Run(scenario, func(t *testing.T) {
			c, b, v := protectedTrade(t, "twap")
			placed, cancelled := len(b.placed), len(b.cancelled)
			// Drive the real scheduler past the second slice's release window.
			start := time.Now().Add(-time.Duration(v.Plan.Orders[1].OffsetSeconds+11) * time.Second)
			v.StartedAt, v.State, v.Error = &start, "running", ""
			if err := c.save(&v); err != nil {
				t.Fatal(err)
			}
			if c.step(context.Background(), "alice", v.ID) {
				t.Fatal("protected position stopped monitoring")
			}
			v, err := c.Get("alice", v.ID)
			if err != nil {
				t.Fatal(err)
			}
			expired := v.Orders[1]
			if v.State != "position_open" || expired.Status != "EXPIRED" || expired.OrderID != "" || expired.Filled != "0" {
				t.Fatalf("late slice did not expire locally: %s %+v", v.State, expired)
			}
			if _, exists := b.orders[expired.ClientID]; exists || len(b.placed) != placed || len(b.cancelled) != cancelled {
				t.Fatal("expiring the late slice changed exchange orders")
			}
			wantState, wantLeases := "position_open", int64(1)
			if scenario != "protected" && scenario != "restarted_protected" {
				b.account.Position = "0"
				for id, order := range b.orders {
					if order.Role != "entry" {
						order.Status = "CANCELED"
						b.orders[id] = order
					}
				}
				wantState, wantLeases = "completed", 0
			}
			switch scenario {
			case "restarted_open_stop":
				stop := b.orders[v.Orders[2].ClientID]
				stop.Status = "NEW"
				b.orders[stop.ClientID] = stop
				wantState, wantLeases = "needs_attention", 1
			case "restarted_unknown_stop":
				delete(b.orders, v.Orders[2].ClientID)
				wantState, wantLeases = "needs_attention", 1
			case "restarted_open_order":
				b.account.OpenOrders = 1
				wantState, wantLeases = "needs_attention", 1
			}
			if scenario != "protected" {
				c.Shutdown()
				c, err = New(c.DB, c.Factory, c.Market)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(c.Shutdown)
			}
			for range 2 {
				out, err := c.CheckStatus(context.Background(), "alice", v.ID)
				if out.State != wantState || (err != nil) != (wantState == "needs_attention") {
					t.Fatalf("state=%s, want %s: %v", out.State, wantState, err)
				}
				if slices.Contains(b.lookedUp, expired.ClientID) {
					t.Fatal("queried a TWAP slice that was never sent")
				}
				var leases int64
				if err := c.DB.Model(&Lease{}).Count(&leases).Error; err != nil {
					t.Fatal(err)
				}
				if leases != wantLeases || len(b.placed) != placed || len(b.cancelled) != cancelled || len(c.workers) != 0 {
					t.Fatalf("reconciliation changed orders/workers or retained wrong lease count: %d, want %d", leases, wantLeases)
				}
			}
		})
	}
}

func TestStatusCheckStillLooksUpUnconfirmedTWAPOrders(t *testing.T) {
	for _, scenario := range []string{"expired_order_id", "expired_fill", "submitting", "expired_protection"} {
		t.Run(scenario, func(t *testing.T) {
			c, b, v := protectedTrade(t, "twap")
			order := &v.Orders[1]
			order.Status = "EXPIRED"
			switch scenario {
			case "expired_order_id":
				order.OrderID = "unconfirmed-native-order"
			case "expired_fill":
				order.Filled = order.Quantity
			case "submitting":
				order.Status = "submitting"
			case "expired_protection":
				order.Status = "scheduled"
				order = &v.Orders[2]
				order.Status, order.OrderID = "EXPIRED", ""
			}
			delete(b.orders, order.ClientID)
			if err := c.save(&v); err != nil {
				t.Fatal(err)
			}
			placed, cancelled := len(b.placed), len(b.cancelled)
			b.lookedUp = nil
			out, err := c.CheckStatus(context.Background(), "alice", v.ID)
			if err == nil || out.State != "needs_attention" {
				t.Fatalf("unconfirmed order was skipped: %s %v", out.State, err)
			}
			if !slices.Contains(b.lookedUp, order.ClientID) {
				t.Fatal("unconfirmed order was not queried at the exchange")
			}
			var leases int64
			if err := c.DB.Model(&Lease{}).Count(&leases).Error; err != nil {
				t.Fatal(err)
			}
			if leases != 1 || len(b.placed) != placed || len(b.cancelled) != cancelled || len(c.workers) != 0 {
				t.Fatal("unconfirmed order released account or changed exchange orders/workers")
			}
		})
	}
}

func TestStatusCheckRequiresReceiptAfterUnconfirmedSubmission(t *testing.T) {
	for _, scenario := range []string{"lost_acknowledgement", "invalid_expired_receipt"} {
		t.Run(scenario, func(t *testing.T) {
			c, b, _, p := setup(t, "limit")
			v := started(t, c, p)
			if scenario == "lost_acknowledgement" {
				b.errorPlace = errors.New("acknowledgement lost after accepting order")
			} else {
				b.invalidExpiryReceipt = true
			}
			c.step(context.Background(), "alice", v.ID)
			v, err := c.Get("alice", v.ID)
			if err != nil {
				t.Fatal(err)
			}
			entry := v.Orders[0]
			if v.State != "needs_attention" || entry.Status != "UNKNOWN" || entry.OrderID != "" || entry.Filled != "0" {
				t.Fatalf("unconfirmed submission did not remain unknown: %s %+v", v.State, entry)
			}
			receipt := b.orders[entry.ClientID]
			delete(b.orders, entry.ClientID)
			b.lookedUp = nil
			out, err := c.CheckStatus(context.Background(), "alice", v.ID)
			if err == nil || out.State != "needs_attention" || !slices.Contains(b.lookedUp, entry.ClientID) {
				t.Fatalf("unconfirmed submission was not queried: %s %v", out.State, err)
			}
			var leases int64
			if err := c.DB.Model(&Lease{}).Count(&leases).Error; err != nil || leases != 1 {
				t.Fatalf("unconfirmed submission lost its reservation: %d %v", leases, err)
			}
			// A later terminal receipt must supply the native ID before release.
			receipt.Status = "EXPIRED"
			b.orders[entry.ClientID] = receipt
			out, err = c.CheckStatus(context.Background(), "alice", v.ID)
			if err != nil || out.State != "completed" || out.Orders[0].OrderID != receipt.OrderID || receipt.OrderID == "" {
				t.Fatalf("confirmed expiry was not reconciled: %s %v", out.State, err)
			}
			if err := c.DB.Model(&Lease{}).Count(&leases).Error; err != nil || leases != 0 {
				t.Fatalf("confirmed expiry retained its reservation: %d %v", leases, err)
			}
			if len(b.placed) != 1 || len(b.cancelled) != 0 || len(c.workers) != 0 {
				t.Fatal("reconciliation changed exchange orders or restarted execution")
			}
		})
	}
}

func TestWorkerClearsTransientSnapshotFailureWithoutNewEntries(t *testing.T) {
	c, b, v := protectedTrade(t, "limit")
	placed := len(b.placed)
	if done := c.step(context.Background(), "alice", v.ID); done {
		t.Fatal("protected open position should remain monitored")
	}
	out, err := c.Get("alice", v.ID)
	if err != nil || out.State != "position_open" || out.Error != "" || len(b.placed) != placed || len(b.cancelled) != 0 {
		t.Fatalf("transient error remained after fresh confirmation: %s %s %v", out.State, out.Error, err)
	}
}

func TestWorkerNeverClearsUnconfirmedReceiptUsingPreviousSnapshot(t *testing.T) {
	c, b, v := protectedTrade(t, "limit")
	b.lookupErr = true
	c.step(context.Background(), "alice", v.ID)
	out, err := c.Get("alice", v.ID)
	if err != nil || out.State != "needs_attention" || out.Error == "" {
		t.Fatalf("stale NEW receipt was treated as fresh protection: %s %s %v", out.State, out.Error, err)
	}
}

func TestWorkerReleasesExternallyClosedTradeOnlyAfterTerminalExitProof(t *testing.T) {
	for _, scenario := range []string{"closed", "unconfirmed", "open_stop", "triggered_stop", "unrelated_open_order"} {
		t.Run(scenario, func(t *testing.T) {
			c, b, v := protectedTrade(t, "limit")
			b.account.Position = "0"
			for id, order := range b.orders {
				if order.Role != "entry" {
					order.Status = "CANCELED"
					b.orders[id] = order
				}
			}
			stop := b.orders[v.Orders[1].ClientID]
			switch scenario {
			case "unconfirmed":
				b.lookupErr = true
			case "open_stop":
				stop.Status = "NEW"
			case "triggered_stop":
				stop.Status = "TRIGGERED"
			case "unrelated_open_order":
				b.account.OpenOrders = 1
			}
			b.orders[stop.ClientID] = stop
			placed, cancelled := len(b.placed), len(b.cancelled)
			done := c.step(context.Background(), "alice", v.ID)
			out, _ := c.Get("alice", v.ID)
			var leases int64
			c.DB.Model(&Lease{}).Count(&leases)
			if scenario == "closed" {
				if !done || out.State != "completed" || leases != 0 || out.Error != "" {
					t.Fatalf("verified external close stayed reserved: %s %d %s", out.State, leases, out.Error)
				}
			} else if done || out.State == "completed" || leases != 1 {
				t.Fatalf("incomplete close released reservation: %s %d", out.State, leases)
			}
			if len(b.placed) != placed || len(b.cancelled) != cancelled {
				t.Fatal("external-close recovery changed exchange orders")
			}
		})
	}
}

func TestAccountActionsDoNotWaitPastContextForBusyRun(t *testing.T) {
	for _, action := range []string{"check", "stop", "reconcile", "execute", "finish_close"} {
		t.Run(action, func(t *testing.T) {
			c, b, _, p := setup(t, "limit")
			v := approved(t, c, p)
			lock := c.runLock(v.ID)
			lock.Lock()
			defer lock.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch action {
				case "check":
					_, err = c.CheckStatus(ctx, "alice", v.ID)
				case "stop":
					_, err = c.StopContext(ctx, "alice", v.ID, v.ApprovalHash)
				case "reconcile":
					_, err = c.ReconcileContext(ctx, "alice", v.ID)
				case "execute":
					_, err = c.Execute(ctx, "alice", v.ID, v.ApprovalHash)
				case "finish_close":
					_, err = c.FinishManualClose(ctx, "alice", v.ID)
				}
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("busy action ignored cancellation")
				}
			case <-time.After(time.Second):
				t.Fatal("account action remained queued after its deadline")
			}
			if len(b.placed) != 0 || len(b.cancelled) != 0 || len(c.workers) != 0 {
				t.Fatal("timed-out action changed exchange orders or started a worker")
			}
		})
	}
}
