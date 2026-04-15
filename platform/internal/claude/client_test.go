package claude

import (
	"strings"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`{"a":1}`, `{"a":1}`},
		{"```json\n{\"a\":1}\n```", `{"a":1}`},
		{"Here is your JSON:\n{\"a\":1}", `{"a":1}`},
		{"```\n[1,2,3]\n```", `[1,2,3]`},
	}
	for _, c := range cases {
		got := extractJSON(c.in)
		if strings.TrimSpace(got) != c.want {
			t.Errorf("extractJSON(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCostUSD(t *testing.T) {
	// 1M input tokens × $3 + 1M output tokens × $15 = $18
	got := CostUSD(1_000_000, 1_000_000)
	if got < 17.99 || got > 18.01 {
		t.Errorf("CostUSD(1M, 1M) = %v, want ~18", got)
	}
	// 0 tokens → 0 cost.
	if CostUSD(0, 0) != 0 {
		t.Errorf("zero cost expected")
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Fatal()
	}
	if truncate("hello world", 5) != "hello...(truncated)" {
		t.Fatal()
	}
}
