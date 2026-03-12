package okx

import (
	"encoding/json"
	"fmt"
	"nofx/logger"
	"strconv"
	"time"
)

// GetPositions gets all positions
func (t *OKXTrader) GetPositions() ([]map[string]interface{}, error) {
	// Check cache
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		t.positionsCacheMutex.RUnlock()
		logger.Infof("✓ Using cached OKX positions")
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	logger.Infof("🔄 Calling OKX API to get positions...")
	data, err := t.doRequest("GET", okxPositionPath+"?instType=SWAP", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %w", err)
	}

	var positions []struct {
		InstId  string `json:"instId"`
		PosSide string `json:"posSide"`
		Pos     string `json:"pos"`
		AvgPx   string `json:"avgPx"`
		MarkPx  string `json:"markPx"`
		Upl     string `json:"upl"`
		Lever   string `json:"lever"`
		LiqPx   string `json:"liqPx"`
		Margin  string `json:"margin"`
		MgnMode string `json:"mgnMode"` // Margin mode: "cross" or "isolated"
		CTime   string `json:"cTime"`   // Position created time (ms)
		UTime   string `json:"uTime"`   // Position last update time (ms)
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse position data: %w", err)
	}

	logger.Infof("🔍 OKX raw positions response: %d positions", len(positions))
	var result []map[string]interface{}
	for _, pos := range positions {
		logger.Infof("🔍 OKX raw position: instId=%s, posSide=%s, pos=%s, mgnMode=%s", pos.InstId, pos.PosSide, pos.Pos, pos.MgnMode)
		contractCount, _ := strconv.ParseFloat(pos.Pos, 64)
		if contractCount == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(pos.AvgPx, 64)
		markPrice, _ := strconv.ParseFloat(pos.MarkPx, 64)
		upl, _ := strconv.ParseFloat(pos.Upl, 64)
		leverage, _ := strconv.ParseFloat(pos.Lever, 64)
		liqPrice, _ := strconv.ParseFloat(pos.LiqPx, 64)

		// Convert symbol format
		symbol := t.convertSymbolBack(pos.InstId)
		logger.Infof("🔍 OKX symbol conversion: %s → %s", pos.InstId, symbol)

		// Determine direction and ensure contractCount is positive
		side := "long"
		if pos.PosSide == "short" {
			side = "short"
		}
		// OKX short position's pos is negative, need to take absolute value
		if contractCount < 0 {
			contractCount = -contractCount
		}

		// Convert contract count to actual position amount (in base asset)
		// positionAmt = contractCount * ctVal
		inst, err := t.getInstrument(symbol)
		posAmt := contractCount
		if err == nil && inst.CtVal > 0 {
			posAmt = contractCount * inst.CtVal
			logger.Debugf("  📊 OKX position %s: contracts=%.4f, ctVal=%.6f, posAmt=%.6f", symbol, contractCount, inst.CtVal, posAmt)
		}

		// Parse timestamps
		cTime, _ := strconv.ParseInt(pos.CTime, 10, 64)
		uTime, _ := strconv.ParseInt(pos.UTime, 10, 64)

		// Default to cross margin mode if not specified
		mgnMode := pos.MgnMode
		if mgnMode == "" {
			mgnMode = "cross"
		}

		posMap := map[string]interface{}{
			"symbol":           symbol,
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": upl,
			"leverage":         leverage,
			"liquidationPrice": liqPrice,
			"side":             side,
			"mgnMode":          mgnMode, // Margin mode: "cross" or "isolated"
			"createdTime":      cTime,   // Position open time (ms)
			"updatedTime":      uTime,   // Position last update time (ms)
		}
		result = append(result, posMap)
	}

	// Update cache
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// InvalidatePositionCache clears the position cache to force fresh data on next call
func (t *OKXTrader) InvalidatePositionCache() {
	t.positionsCacheMutex.Lock()
	t.cachedPositions = nil
	t.positionsCacheTime = time.Time{}
	t.positionsCacheMutex.Unlock()
}

// getInstrument gets instrument info
func (t *OKXTrader) getInstrument(symbol string) (*OKXInstrument, error) {
	instId := t.convertSymbol(symbol)

	// Check cache
	t.instrumentsCacheMutex.RLock()
	if inst, ok := t.instrumentsCache[instId]; ok && time.Since(t.instrumentsCacheTime) < 5*time.Minute {
		t.instrumentsCacheMutex.RUnlock()
		return inst, nil
	}
	t.instrumentsCacheMutex.RUnlock()

	// Get instrument info
	path := fmt.Sprintf("%s?instType=SWAP&instId=%s", okxInstrumentsPath, instId)
	data, err := t.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var instruments []struct {
		InstId   string `json:"instId"`
		CtVal    string `json:"ctVal"`
		CtMult   string `json:"ctMult"`
		LotSz    string `json:"lotSz"`
		MinSz    string `json:"minSz"`
		MaxMktSz string `json:"maxMktSz"` // Maximum market order size
		TickSz   string `json:"tickSz"`
		CtType   string `json:"ctType"`
	}

	if err := json.Unmarshal(data, &instruments); err != nil {
		return nil, err
	}

	if len(instruments) == 0 {
		return nil, fmt.Errorf("instrument info not found: %s", instId)
	}

	inst := instruments[0]
	ctVal, _ := strconv.ParseFloat(inst.CtVal, 64)
	ctMult, _ := strconv.ParseFloat(inst.CtMult, 64)
	lotSz, _ := strconv.ParseFloat(inst.LotSz, 64)
	minSz, _ := strconv.ParseFloat(inst.MinSz, 64)
	maxMktSz, _ := strconv.ParseFloat(inst.MaxMktSz, 64)
	tickSz, _ := strconv.ParseFloat(inst.TickSz, 64)

	instrument := &OKXInstrument{
		InstID:   inst.InstId,
		CtVal:    ctVal,
		CtMult:   ctMult,
		LotSz:    lotSz,
		MinSz:    minSz,
		MaxMktSz: maxMktSz,
		TickSz:   tickSz,
		CtType:   inst.CtType,
	}

	// Update cache
	t.instrumentsCacheMutex.Lock()
	t.instrumentsCache[instId] = instrument
	t.instrumentsCacheTime = time.Now()
	t.instrumentsCacheMutex.Unlock()

	return instrument, nil
}
