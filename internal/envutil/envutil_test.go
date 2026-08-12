package envutil

import (
	"testing"
)

func TestEnvMapIncludesEnvironment(t *testing.T) {
	t.Setenv("AM_TEST_KEY", "hello")
	m := EnvMap()
	if m["AM_TEST_KEY"] != "hello" {
		t.Fatalf("expected AM_TEST_KEY=hello, got %q", m["AM_TEST_KEY"])
	}
}

func TestEnvListRoundTrips(t *testing.T) {
	input := map[string]string{"A": "1", "B": "2"}
	list := EnvList(input)
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	seen := map[string]bool{}
	for _, entry := range list {
		seen[entry] = true
	}
	if !seen["A=1"] || !seen["B=2"] {
		t.Fatalf("unexpected list: %v", list)
	}
}

func TestParsePassthrough(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"USER,LOGNAME", 2},
		{"USER LOGNAME", 2},
		{"USER, LOGNAME", 2},
		{"", 0},
		{"  ", 0},
	}
	for _, tc := range cases {
		got := ParsePassthrough(tc.input)
		if len(got) != tc.want {
			t.Errorf("ParsePassthrough(%q) = %d items, want %d", tc.input, len(got), tc.want)
		}
	}
}

func TestIsUnsafe(t *testing.T) {
	if IsUnsafe(true) != true {
		t.Error("flag=true should be unsafe")
	}
	if IsUnsafe(false) != false {
		t.Error("flag=false without env should be safe")
	}
	t.Setenv("AM_UNSAFE", "1")
	if IsUnsafe(false) != true {
		t.Error("AM_UNSAFE=1 should be unsafe")
	}
}
