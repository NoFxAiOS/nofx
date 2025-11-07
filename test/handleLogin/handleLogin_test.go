// @Target(handleLogin)
package handleLogin

import (
	"nofx/test/harness"
	"testing"
)

// HandleLoginTest 嵌入 BaseTest，可按需重写 Before/After 钩子
type HandleLoginTest struct {
	harness.BaseTest
}

func (rt *HandleLoginTest) Before(t *testing.T) {
	rt.BaseTest.Before(t)
	if rt.Env != nil {
		t.Logf("TestEnv API URL: %s", rt.Env.URL())
	} else {
		t.Log("Warning: Env is nil in Before")
	}
}

func (rt *HandleLoginTest) After(t *testing.T) {
	// no-op
}

// @RunWith(case02)
func TestHandleLogin(t *testing.T) {
	rt := &HandleLoginTest{}
	harness.RunCase(t, rt)
}
