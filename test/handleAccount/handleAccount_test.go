// See: api/server.go -> func (s *Server) handleAccount(c *gin.Context)
// @Target(handleAccount)
package handleaccount

import (
	"net/http"
	"nofx/test/harness"
	"testing"

	"github.com/jarcoal/httpmock"
)

// HandleAccountTest 嵌入 BaseTest，可按需重写 Before/After 钩子
type HandleAccountTest struct {
	harness.BaseTest
}

func (rt *HandleAccountTest) Before(t *testing.T) {
	rt.BaseTest.Before(t)
	if rt.Env != nil {
		t.Logf("TestEnv API URL: %s", rt.Env.URL())
	} else {
		t.Log("Warning: Env is nil in Before")
	}
}

func (rt *HandleAccountTest) After(t *testing.T) {
	// no-op
}

func TestHandleAccount(t *testing.T) {
	// 启用 httpmock
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	// If a request isn't registered with httpmock, forward it to the real transport
	httpmock.RegisterNoResponder(func(req *http.Request) (*http.Response, error) {
		return httpmock.InitialTransport.RoundTrip(req)
	})
	// 模拟特定的 API 路径
	httpmock.RegisterResponder("GET", "https://fapi.binance.com/fapi/v2/positionRisk",
		httpmock.NewStringResponder(200, `[
            {
                "symbol": "BTCUSDT",
                "positionAmt": "0",
                "entryPrice": "40000.0"
            }
        ]`))

	httpmock.RegisterResponder("GET", "https://fapi.binance.com/fapi/v2/account",
		httpmock.NewStringResponder(200, `{
            "totalWalletBalance": "0",
            "availableBalance": "0",
            "totalUnrealizedProfit": "0"
        }`))

	rt := &HandleAccountTest{}
	harness.RunCase(t, rt)
}
