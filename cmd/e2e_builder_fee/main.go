package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"nofx/config"
	nofxcrypto "nofx/crypto"
	"nofx/store"
	hltrader "nofx/trader/hyperliquid"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type clearinghouseState struct {
	CrossMarginSummary struct {
		AccountValue string `json:"accountValue"`
	} `json:"crossMarginSummary"`
	Withdrawable    string `json:"withdrawable"`
	AssetPositions []struct {
		Position struct {
			Coin          string `json:"coin"`
			Szi           string `json:"szi"`
			EntryPx       string `json:"entryPx"`
			PositionValue string `json:"positionValue"`
		} `json:"position"`
	} `json:"assetPositions"`
}

func fetchState(wallet string) (*clearinghouseState, error) {
	body := strings.NewReader(fmt.Sprintf(`{"type":"clearinghouseState","user":%q}`, wallet))
	resp, err := http.Post("https://api.hyperliquid.xyz/info", "application/json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var state clearinghouseState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func positionSize(state *clearinghouseState, coin string) float64 {
	for _, ap := range state.AssetPositions {
		if strings.EqualFold(ap.Position.Coin, coin) {
			v, _ := strconv.ParseFloat(ap.Position.Szi, 64)
			return v
		}
	}
	return 0
}

func main() {
	_ = godotenv.Load()
	config.Init()
	cryptoService, err := nofxcrypto.NewCryptoService()
	if err != nil {
		panic(err)
	}
	nofxcrypto.SetGlobalCryptoService(cryptoService)
	cfg := config.Get()
	st, err := store.NewWithConfig(store.DBConfig{Type: store.DBTypeSQLite, Path: cfg.DBPath})
	if err != nil {
		panic(err)
	}
	defer st.Close()

	var ex store.Exchange
	if err := st.GormDB().Where("exchange_type = ? AND enabled = ? AND hyperliquid_wallet_addr <> ''", "hyperliquid", true).First(&ex).Error; err != nil {
		panic(fmt.Errorf("no enabled Hyperliquid exchange with wallet/private key found: %w", err))
	}
	if strings.TrimSpace(string(ex.APIKey)) == "" {
		panic("Hyperliquid exchange has empty decrypted agent private key")
	}

	fmt.Printf("E2E exchange=%s account=%s wallet=%s testnet=%v builderApprovedFlag=%v\n", ex.ID, ex.AccountName, ex.HyperliquidWalletAddr, ex.Testnet, ex.HyperliquidBuilderApproved)
	before, err := fetchState(ex.HyperliquidWalletAddr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("BEFORE accountValue=%s withdrawable=%s HOOD_szi=%.6f\n", before.CrossMarginSummary.AccountValue, before.Withdrawable, positionSize(before, "xyz:HOOD"))

	tr, err := hltrader.NewHyperliquidTrader(string(ex.APIKey), ex.HyperliquidWalletAddr, false, ex.HyperliquidUnifiedAcct)
	if err != nil {
		panic(err)
	}

	const symbol = "HOOD-USDC"
	const qty = 0.15
	fmt.Printf("OPEN_LONG symbol=%s qty=%.3f builderRequired=true\n", symbol, qty)
	if _, err := tr.OpenLong(symbol, qty, 1); err != nil {
		panic(fmt.Errorf("open long failed: %w", err))
	}
	time.Sleep(2 * time.Second)
	mid, _ := fetchState(ex.HyperliquidWalletAddr)
	pos := positionSize(mid, "xyz:HOOD")
	fmt.Printf("AFTER_OPEN HOOD_szi=%.6f\n", pos)
	closeQty := qty
	if pos > 0 && pos < closeQty {
		closeQty = pos
	}
	if closeQty > 0 {
		fmt.Printf("CLOSE_LONG symbol=%s qty=%.6f builderRequired=true\n", symbol, closeQty)
		if _, err := tr.CloseLong(symbol, closeQty); err != nil {
			panic(fmt.Errorf("close long failed; manual intervention may be needed for %s size %.6f: %w", symbol, closeQty, err))
		}
	}
	time.Sleep(2 * time.Second)
	after, err := fetchState(ex.HyperliquidWalletAddr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("AFTER_CLOSE accountValue=%s withdrawable=%s HOOD_szi=%.6f\n", after.CrossMarginSummary.AccountValue, after.Withdrawable, positionSize(after, "xyz:HOOD"))
	fmt.Fprintln(os.Stdout, "E2E_BUILDER_FEE_REAL_XYZ_STOCK_TRADE_DONE")
}
