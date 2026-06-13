package nofxos

import (
	"os"
	"strings"

	"nofx/logger"
)

// ResolveClient returns a nofxos data client, routed through the claw402
// x402 payment gateway when a wallet key is available. Resolution order:
// the explicit walletKey argument, then the CLAW402_WALLET_KEY environment
// variable, then the direct nofxos.ai client with the default auth key.
func ResolveClient(walletKey string) *Client {
	walletKey = strings.TrimSpace(walletKey)
	if walletKey == "" {
		walletKey = strings.TrimSpace(os.Getenv("CLAW402_WALLET_KEY"))
	}
	client := NewClient(DefaultBaseURL, DefaultAuthKey)
	if walletKey == "" {
		return client
	}

	claw402URL := strings.TrimSpace(os.Getenv("CLAW402_URL"))
	claw402Client, err := NewClaw402DataClient(claw402URL, walletKey, &logger.MCPLogger{})
	if err != nil {
		logger.Warnf("⚠️ Failed to init claw402 data client: %v (using direct nofxos.ai)", err)
		return client
	}
	client.SetClaw402(claw402Client)
	return client
}
