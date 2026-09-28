package song

import (
	"strings"
	"testing"
)

func TestXingyuConversion(t *testing.T) {
	raw := []byte(`{"lines":[{"index":0,"text":"再唱 AI","start":1,"end":3,"status":"aligned","tokens":[{"text":"再唱","start":1,"end":2},{"text":"AI","start":2,"end":3,"estimated":true}]},{"index":1,"text":"再唱 AI","start":5,"end":7,"status":"aligned"}]}`)
	tl, issues, e := ConvertAlignment(raw, Candidate{Duration: 9}, []string{"再唱 AI", "再唱 AI"})
	if e != nil || len(issues) != 0 || len(tl.Lines) != 2 || tl.Lines[0].ID == tl.Lines[1].ID || len(tl.Lines[0].Words) != 1 || len(tl.Warnings) == 0 {
		t.Fatal(tl, issues, e)
	}
	raw = []byte(`{"lines":[{"index":0,"text":"词","start":null,"end":3,"status":"missing_timestamps"}]}`)
	_, issues, e = ConvertAlignment(raw, Candidate{Duration: 9}, []string{"词"})
	if e != nil || len(issues) == 0 || !strings.Contains(issues[0], "时间") {
		t.Fatal(issues, e)
	}
}
