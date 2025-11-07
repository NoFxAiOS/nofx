// See: api/server.go -> func (s *Server) handlePositions(c *gin.Context)
// @Target(handlePositions)
package handlepositions

import (
	"testing"

	"nofx/test/harness"
)

// HandlePositionsTest 嵌入 BaseTest，可按需重写 Before/After 钩子
type HandlePositionsTest struct {
	harness.BaseTest
}

func (rt *HandlePositionsTest) Before(t *testing.T) {
	rt.BaseTest.Before(t)
	if rt.Env != nil {
		t.Logf("TestEnv API URL: %s", rt.Env.URL())
	} else {
		t.Log("Warning: Env is nil in Before")
	}
}

func (rt *HandlePositionsTest) After(t *testing.T) {
	// no-op
}

// @RunWith(case01)
func TestHandlePositions(t *testing.T) {
	rt := &HandlePositionsTest{}
	harness.RunCase(t, rt)
}
