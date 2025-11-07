// See: api/server.go -> func (s *Server) handleRegister(c *gin.Context)
// @Target(handleRegister)
package handleregister

import (
	"nofx/test/harness"
	"testing"
)

// HandleRegisterTest 嵌入 BaseTest，可按需重写 Before/After 钩子
type HandleRegisterTest struct {
	harness.BaseTest
}

func (rt *HandleRegisterTest) Before(t *testing.T) {
	rt.BaseTest.Before(t)
	if rt.Env != nil {
		t.Logf("TestEnv API URL: %s", rt.Env.URL())
	} else {
		t.Log("Warning: Env is nil in Before")
	}
}

func (rt *HandleRegisterTest) After(t *testing.T) {
	// no-op
}

// @RunWith(case01)
func TestHandleRegister(t *testing.T) {
	rt := &HandleRegisterTest{}
	harness.RunCase(t, rt)
}
