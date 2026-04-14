package gmgn

import "strings"

const (
	ChainSOL  = "sol"
	ChainBSC  = "bsc"
	ChainBASE = "base"
)

type ChainConfig struct {
	Chain             string
	USDCAddress       string
	USDCSymbol        string
	NativeTokenSymbol string
	NativeTokenAddr   string
	MinGasBuffer      float64
}

var chainConfigs = map[string]ChainConfig{
	ChainSOL: {
		Chain:             ChainSOL,
		USDCAddress:       "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		USDCSymbol:        "USDC",
		NativeTokenSymbol: "SOL",
		NativeTokenAddr:   "So11111111111111111111111111111111111111112",
		MinGasBuffer:      0.01,
	},
	ChainBSC: {
		Chain:             ChainBSC,
		USDCAddress:       "0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d",
		USDCSymbol:        "USDC",
		NativeTokenSymbol: "BNB",
		NativeTokenAddr:   "0x0000000000000000000000000000000000000000",
		MinGasBuffer:      0.002,
	},
	ChainBASE: {
		Chain:             ChainBASE,
		USDCAddress:       "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
		USDCSymbol:        "USDC",
		NativeTokenSymbol: "ETH",
		NativeTokenAddr:   "0x0000000000000000000000000000000000000000",
		MinGasBuffer:      0.0003,
	},
}

func GetChainConfig(chain string) (ChainConfig, bool) {
	cfg, ok := chainConfigs[strings.ToLower(strings.TrimSpace(chain))]
	return cfg, ok
}

func NormalizeChain(chain string) string {
	return strings.ToLower(strings.TrimSpace(chain))
}
