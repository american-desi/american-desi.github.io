package generator

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Best Budgeting Apps 2026":      "best-budgeting-apps-2026",
		"Personal finance tools & apps": "personal-finance-tools-apps",
		"   Spaces   Everywhere   ":     "spaces-everywhere",
		"Café & Crème":                  "caf-cr-me",
	}
	for in, want := range cases {
		got := Slugify(in)
		if got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHashDeterministic(t *testing.T) {
	a := Hash("site", "keyword", "pillar")
	b := Hash("site", "keyword", "pillar")
	if a != b {
		t.Fatal("hash should be deterministic")
	}
	c := Hash("site", "keyword", "supporting")
	if a == c {
		t.Fatal("different inputs should hash differently")
	}
	if len(a) != 16 {
		t.Fatalf("expected 16-char hash, got %d", len(a))
	}
}
