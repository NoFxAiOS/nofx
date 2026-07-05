package trader

import (
	"time"
)

// Runtime health state written by the run-loop goroutine and read by the API
// layer (GetStatus). Everything here goes through runtimeHealthMu so the
// dashboard can poll without racing the trading loop.

// AI fee wallet health statuses exposed via GetStatus as "ai_wallet_status".
const (
	AIWalletStatusOK      = "ok"
	AIWalletStatusLow     = "low"
	AIWalletStatusEmpty   = "empty"
	AIWalletStatusUnknown = "unknown"
)

// aiWalletLowThresholdUSDC mirrors api.MinAIFeeUSDC: below this the wallet
// cannot reliably pay for the next AI/data calls.
const aiWalletLowThresholdUSDC = 1.0

func (at *AutoTrader) setSafeMode(active bool, reason string) {
	at.runtimeHealthMu.Lock()
	at.safeMode = active
	at.safeModeReason = reason
	at.runtimeHealthMu.Unlock()
}

func (at *AutoTrader) isSafeMode() bool {
	at.runtimeHealthMu.RLock()
	defer at.runtimeHealthMu.RUnlock()
	return at.safeMode
}

func (at *AutoTrader) safeModeState() (bool, string) {
	at.runtimeHealthMu.RLock()
	defer at.runtimeHealthMu.RUnlock()
	return at.safeMode, at.safeModeReason
}

// setAIWalletHealth records a fresh balance observation for the claw402 wallet.
func (at *AutoTrader) setAIWalletHealth(balance float64) {
	status := AIWalletStatusOK
	switch {
	case balance <= 0:
		status = AIWalletStatusEmpty
	case balance < aiWalletLowThresholdUSDC:
		status = AIWalletStatusLow
	}

	at.runtimeHealthMu.Lock()
	at.aiWalletStatus = status
	at.aiWalletBalanceUSDC = balance
	at.aiWalletCheckedAt = time.Now().UTC()
	at.runtimeHealthMu.Unlock()
}

// markAIWalletHealthUnknown keeps the last observed balance but flags that the
// current reading could not be refreshed (e.g. Base RPC unreachable).
func (at *AutoTrader) markAIWalletHealthUnknown() {
	at.runtimeHealthMu.Lock()
	at.aiWalletStatus = AIWalletStatusUnknown
	at.aiWalletCheckedAt = time.Now().UTC()
	at.runtimeHealthMu.Unlock()
}

// markAIWalletEmptyFromPayment records a payment-layer rejection: the wallet
// definitively could not cover an AI call.
func (at *AutoTrader) markAIWalletEmptyFromPayment(balance float64) {
	at.runtimeHealthMu.Lock()
	at.aiWalletStatus = AIWalletStatusEmpty
	at.aiWalletBalanceUSDC = balance
	at.aiWalletCheckedAt = time.Now().UTC()
	at.runtimeHealthMu.Unlock()
}

func (at *AutoTrader) aiWalletHealth() (status string, balance float64, checkedAt time.Time) {
	at.runtimeHealthMu.RLock()
	defer at.runtimeHealthMu.RUnlock()
	return at.aiWalletStatus, at.aiWalletBalanceUSDC, at.aiWalletCheckedAt
}
