package stockengine

import (
	"errors"
	"strings"
	"testing"
	"time"

	"nofx/mcp"
)

func TestParseDecisions(t *testing.T) {
	tests := []struct {
		name, response        string
		symbol, action, entry string
		count                 int
		fail                  bool
	}{
		{"fenced", "Summary\n```json\n[{\"symbol\":\"aaplbusdt\",\"action\":\"OPEN_LONG\",\"entry_type\":\"LIMIT\"}]\n```\nTrailing prose", "AAPLBUSDT", ActionOpenLong, EntryLimit, 1, false},
		{"raw trailing", `Thoughts [{"symbol":" MSFTbusdt ","action":" wait "}] done`, "MSFTBUSDT", ActionWait, "", 1, false},
		{"unknown dropped", `[{"symbol":"A","action":"open_short"},{"symbol":"aaplbusdt","action":"hold"}]`, "AAPLBUSDT", ActionHold, "", 1, false},
		{"last array", `[{"symbol":"OLD","action":"wait"}] prose [{"symbol":"aaplbusdt","action":"open_long","reasoning":"brackets [inside] and escaped quote \""}] trailing`, "AAPLBUSDT", ActionOpenLong, "", 1, false},
		{"last valid array", `[{"symbol":"aaplbusdt","action":"wait"}] junk [not valid JSON]`, "AAPLBUSDT", ActionWait, "", 1, false},
		{"empty", `Summary [] trailing`, "", "", "", 0, false},
		{"all unknown", `[{"symbol":"A","action":"sell_short"}]`, "", "", "", 0, false},
		{"absent", `No decision array`, "", "", "", 0, true},
		{"malformed", `[{"symbol":"A"`, "", "", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ParseDecisions(tt.response)
			if tt.fail {
				if err == nil {
					t.Fatal("expected parse error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(d) != tt.count {
				t.Fatal(d)
			}
			if len(d) > 0 && (d[0].Symbol != tt.symbol || d[0].Action != tt.action || d[0].EntryType != tt.entry) {
				t.Fatal(d)
			}
		})
	}
}

type fakeClient struct {
	response     string
	err          error
	system, user string
	calls        int
}

var _ mcp.AIClient = (*fakeClient)(nil)

func (*fakeClient) SetAPIKey(string, string, string) {}
func (*fakeClient) SetTimeout(time.Duration)         {}
func (f *fakeClient) CallWithMessages(system, user string) (string, error) {
	f.calls++
	f.system, f.user = system, user
	return f.response, f.err
}
func (*fakeClient) CallWithRequest(*mcp.Request) (string, error) { return "", errors.New("unused") }
func (*fakeClient) CallWithRequestStream(*mcp.Request, func(string)) (string, error) {
	return "", errors.New("unused")
}
func (*fakeClient) CallWithRequestFull(*mcp.Request) (*mcp.LLMResponse, error) {
	return nil, errors.New("unused")
}

func TestDecideFakeClient(t *testing.T) {
	c := testContext()
	c.Now = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeClient{response: `Summary [{"symbol":"aaplbusdt","action":"open_long","stop_loss":96,"confidence":85,"thesis":"t","invalidation":"i"},{"symbol":"MSFTBUSDT","action":"open_long","stop_loss":100,"confidence":85,"thesis":"t","invalidation":"i"},{"symbol":"QQQBUSDT","action":"wait"}]`}
	r, err := Decide(c, f, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Decisions) != 3 || len(r.Verdicts) != 3 {
		t.Fatal(r)
	}
	assertCode(t, r.Verdicts[0], "")
	assertCode(t, r.Verdicts[1], "STOP_INVALID") // repairable: one repair call, same answer, still rejected
	assertCode(t, r.Verdicts[2], "")
	if f.calls != 2 || len(r.Repaired) != 1 || r.Repaired[0] != "MSFTBUSDT" {
		t.Fatalf("calls=%d repaired=%v", f.calls, r.Repaired)
	}
	if r.SystemPrompt == "" || r.UserPrompt == "" || r.SystemPrompt != f.system || r.RepairPrompt != f.user || r.RawResponse != f.response {
		t.Fatal("result/call mismatch")
	}
	if r.DurationMs < 0 || r.DurationMs > 1000 {
		t.Fatalf("duration used trading clock: %d", r.DurationMs)
	}
}

func TestDecideErrorsPreserveResponse(t *testing.T) {
	for _, callErr := range []error{nil, errors.New("fake failure")} {
		f := &fakeClient{response: "unparseable raw response", err: callErr}
		r, err := Decide(testContext(), f, "zh")
		if err == nil || r == nil || r.RawResponse != f.response || r.SystemPrompt == "" || len(r.Verdicts) != 0 {
			t.Fatalf("result=%+v error=%v", r, err)
		}
	}
}

func TestParseResponseObject(t *testing.T) {
	obj := `{"reasoning":" Risk-on; keep two names. ","market_context":{"overall_sentiment":" Bullish ","key_factors":["SPY > EMA50","vol [low]"]},` +
		`"decisions":[{"symbol":"aaplbusdt","action":"OPEN_LONG","entry_type":"LIMIT","stop_loss":96,"confidence":85,` +
		`"thesis":"  steps [1,2] and {\"brace\"} ","invalidation":" close < 96 "},{"symbol":"x","action":"short"}]}`
	tests := []struct{ name, response string }{
		{"bare", obj},
		{"fenced with prose", "Thinking [a] {not json}\n```json\n" + obj + "\n```\nHope that helps."},
		{"earlier object ignored", `{"decisions":[{"symbol":"OLD","action":"wait"}]} then ` + obj},
		{"object without decisions ignored", obj + ` {"note":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reasoning, mc, ds, err := ParseResponse(tt.response)
			if err != nil {
				t.Fatal(err)
			}
			if reasoning != "Risk-on; keep two names." {
				t.Fatalf("reasoning %q", reasoning)
			}
			if mc == nil || mc.OverallSentiment != "bullish" || len(mc.KeyFactors) != 2 || mc.KeyFactors[1] != "vol [low]" {
				t.Fatalf("market context %+v", mc)
			}
			if len(ds) != 1 {
				t.Fatalf("unknown action must be dropped: %+v", ds)
			}
			d := ds[0]
			if d.Symbol != "AAPLBUSDT" || d.Action != ActionOpenLong || d.EntryType != EntryLimit || d.Confidence != 85 ||
				d.Thesis != `steps [1,2] and {"brace"}` || d.Invalidation != "close < 96" {
				t.Fatalf("decision %+v", d)
			}
		})
	}
}

func TestParseResponseUnknownSentimentKeptAndLegacyArray(t *testing.T) {
	_, mc, ds, err := ParseResponse(`{"market_context":{"overall_sentiment":"Choppy"},"decisions":[]}`)
	if err != nil || mc == nil || mc.OverallSentiment != "choppy" || len(ds) != 0 {
		t.Fatalf("%+v %v %v", mc, ds, err)
	}
	reasoning, mc, ds, err := ParseResponse(`Summary [{"symbol":"aaplbusdt","action":"hold"}] done`)
	if err != nil || reasoning != "" || mc != nil || len(ds) != 1 || ds[0].Symbol != "AAPLBUSDT" {
		t.Fatalf("legacy fallback: %q %+v %v %v", reasoning, mc, ds, err)
	}
	for _, bad := range []string{"no json at all", `{"reasoning":"x"}`, `{"decisions":[{"symbol":"A"`} {
		if _, _, _, err := ParseResponse(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

// scriptedClient answers each call with the next scripted response/error.
type scriptedClient struct {
	fakeClient
	responses []string
	errs      []error
	users     []string
}

func (s *scriptedClient) CallWithMessages(system, user string) (string, error) {
	i := s.calls
	s.calls++
	s.users = append(s.users, user)
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	if i >= len(s.responses) {
		return "", errors.New("unexpected extra call")
	}
	return s.responses[i], err
}

func obj(decisions string) string {
	return `{"reasoning":"r","market_context":{"overall_sentiment":"neutral","key_factors":["k"]},"decisions":[` + decisions + `]}`
}

const (
	aaplGood = `{"symbol":"AAPLBUSDT","action":"open_long","entry_type":"market","stop_loss":96,"confidence":85,"thesis":"t","invalidation":"i"}`
	// ATR is 2, band [1.5,3.0]: stop 99 is 0.5×ATR.
	msftTight = `{"symbol":"MSFTBUSDT","action":"open_long","entry_type":"market","stop_loss":99,"confidence":85,"thesis":"t","invalidation":"i"}`
	msftFixed = `{"symbol":"MSFTBUSDT","action":"open_long","entry_type":"market","stop_loss":96,"confidence":85,"thesis":"t","invalidation":"i"}`
	qqqWait   = `{"symbol":"QQQBUSDT","action":"wait"}`
)

func TestDecideNoRepairWhenAllGood(t *testing.T) {
	c := &scriptedClient{responses: []string{obj(aaplGood + "," + qqqWait)}}
	r, err := Decide(testContext(), c, "en")
	if err != nil || c.calls != 1 {
		t.Fatalf("calls=%d err=%v", c.calls, err)
	}
	assertCode(t, r.Verdicts[0], "")
	if r.Reasoning != "r" || r.MarketContext == nil || r.MarketContext.OverallSentiment != "neutral" {
		t.Fatalf("%q %+v", r.Reasoning, r.MarketContext)
	}
	if r.RepairPrompt != "" || r.RepairRawResponse != "" || len(r.Repaired) != 0 || r.RepairDurationMs != 0 {
		t.Fatalf("no repair expected: %+v", r)
	}
}

func TestDecideRepairsStopOutOfBand(t *testing.T) {
	c := &scriptedClient{responses: []string{obj(aaplGood + "," + msftTight + "," + qqqWait), obj(msftFixed)}}
	r, err := Decide(testContext(), c, "en")
	if err != nil {
		t.Fatal(err)
	}
	if c.calls != 2 {
		t.Fatalf("exactly one repair call expected, got %d", c.calls)
	}
	if len(r.Verdicts) != 3 || len(r.Decisions) != 3 {
		t.Fatalf("merged list keeps all decisions: %+v", r.Verdicts)
	}
	for _, v := range r.Verdicts {
		assertCode(t, v, "")
	}
	if r.Decisions[1].Symbol != "MSFTBUSDT" || r.Decisions[1].StopLoss != 96 {
		t.Fatalf("repaired decision not merged in place: %+v", r.Decisions)
	}
	if len(r.Repaired) != 1 || r.Repaired[0] != "MSFTBUSDT" || r.RepairRawResponse != obj(msftFixed) {
		t.Fatalf("repair bookkeeping: %+v", r)
	}
	repair := c.users[1]
	for _, want := range []string{r.UserPrompt, "## REPAIR REQUEST (one attempt)", "Your previous response:", obj(aaplGood + "," + msftTight + "," + qqqWait),
		"- MSFTBUSDT open_long: STOP_OUT_OF_BAND (stop must be within [1.50, 3.00]×ATR(1d) = [94.0000, 97.0000]", "decisions ONLY for these symbols"} {
		if !strings.Contains(repair, want) {
			t.Fatalf("repair prompt missing %q:\n%s", want, repair)
		}
	}
	if strings.Contains(repair, "- AAPLBUSDT") {
		t.Fatal("accepted decisions must not be in the repair request")
	}
	if r.RepairPrompt != repair {
		t.Fatal("RepairPrompt must be the prompt sent")
	}
	// zh variant
	c2 := &scriptedClient{responses: []string{obj(msftTight), obj(msftFixed)}}
	if _, err := Decide(testContext(), c2, "zh"); err != nil || !strings.Contains(c2.users[1], "## 修复请求（仅此一次）") || !strings.Contains(c2.users[1], "- MSFTBUSDT open_long: STOP_OUT_OF_BAND") {
		t.Fatalf("zh repair prompt: %v\n%s", err, c2.users[len(c2.users)-1])
	}
}

func TestDecideRepairsMissingThesisAndStillRepairsOnlyOnce(t *testing.T) {
	noThesis := `{"symbol":"AAPLBUSDT","action":"open_long","stop_loss":96,"confidence":85}`
	c := &scriptedClient{responses: []string{obj(noThesis), obj(noThesis)}}
	r, err := Decide(testContext(), c, "en")
	if err != nil || c.calls != 2 {
		t.Fatalf("calls=%d err=%v", c.calls, err)
	}
	assertCode(t, r.Verdicts[0], "THESIS_REQUIRED") // second attempt still bad → rejected, no third call
}

func TestDecideLowConfidenceIsNotRepaired(t *testing.T) {
	low := `{"symbol":"AAPLBUSDT","action":"open_long","stop_loss":96,"confidence":79,"thesis":"t","invalidation":"i"}`
	c := &scriptedClient{responses: []string{obj(low)}}
	r, err := Decide(testContext(), c, "en")
	if err != nil || c.calls != 1 {
		t.Fatalf("calls=%d err=%v", c.calls, err)
	}
	assertCode(t, r.Verdicts[0], "LOW_CONFIDENCE")
	// Low confidence together with a bad stop is still not repairable.
	both := `{"symbol":"AAPLBUSDT","action":"open_long","stop_loss":99,"confidence":50,"thesis":"t","invalidation":"i"}`
	c = &scriptedClient{responses: []string{obj(both)}}
	if r, _ = Decide(testContext(), c, "en"); c.calls != 1 {
		t.Fatalf("calls=%d %+v", c.calls, r.Verdicts)
	}
}

func TestDecideRepairIgnoresOtherSymbols(t *testing.T) {
	// The repair answer also tries to open AAPL differently and adds a QQQ open: both ignored.
	other := `{"symbol":"AAPLBUSDT","action":"close_long"},{"symbol":"QQQBUSDT","action":"open_long","stop_loss":96,"confidence":99,"thesis":"t","invalidation":"i"}`
	c := &scriptedClient{responses: []string{obj(aaplGood + "," + msftTight + "," + qqqWait), obj(msftFixed + "," + other)}}
	r, err := Decide(testContext(), c, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Decisions) != 3 {
		t.Fatalf("%+v", r.Decisions)
	}
	if r.Decisions[0].Action != ActionOpenLong || r.Decisions[2].Action != ActionWait {
		t.Fatalf("non-rejected decisions must be untouched: %+v", r.Decisions)
	}
	for _, v := range r.Verdicts {
		assertCode(t, v, "")
	}
}

func TestDecideRepairOmittedSymbolKeepsOriginalRejection(t *testing.T) {
	c := &scriptedClient{responses: []string{obj(aaplGood + "," + msftTight), obj(`{"symbol":"QQQBUSDT","action":"wait"}`)}}
	r, err := Decide(testContext(), c, "en")
	if err != nil || len(r.Verdicts) != 2 {
		t.Fatalf("%v %+v", err, r)
	}
	assertCode(t, r.Verdicts[1], "STOP_OUT_OF_BAND")
}

func TestDecideRepairFailureKeepsFirstVerdicts(t *testing.T) {
	first := obj(aaplGood + "," + msftTight)
	for name, c := range map[string]*scriptedClient{
		"call error":  {responses: []string{first, ""}, errs: []error{nil, errors.New("boom")}},
		"unparseable": {responses: []string{first, "sorry, I cannot"}},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Decide(testContext(), c, "en")
			if err != nil {
				t.Fatalf("repair failure must not fail the cycle: %v", err)
			}
			assertCode(t, r.Verdicts[0], "")
			assertCode(t, r.Verdicts[1], "STOP_OUT_OF_BAND")
			if r.RepairRawResponse == "" || len(r.Repaired) != 1 {
				t.Fatalf("repair error text expected: %+v", r)
			}
			if name == "call error" && !strings.Contains(r.RepairRawResponse, "boom") {
				t.Fatal(r.RepairRawResponse)
			}
		})
	}
}

func TestConfidenceParsingTolerant(t *testing.T) {
	cases := map[string]int{`85`: 85, `85.5`: 86, `0.85`: 85, `"90"`: 90, `"80%"`: 80, `1`: 100, `0`: 0, `150`: 0, `-3`: 0, `"high"`: 0, `null`: 0}
	for raw, want := range cases {
		_, _, ds, err := ParseResponse(`{"decisions":[{"symbol":"AAPLBUSDT","action":"wait","confidence":` + raw + `}]}`)
		if err != nil || len(ds) != 1 || ds[0].Confidence != want {
			t.Errorf("confidence %s → %+v err=%v, want %d", raw, ds, err, want)
		}
	}
}
