package generator

import (
	"strings"
	"testing"
)

func TestSanitizeHTML(t *testing.T) {
	dirty := `<h1>ok</h1><script>alert(1)</script><p onclick="bad()">x</p><a href="javascript:alert(2)">y</a>`
	clean := sanitizeHTML(dirty)
	if strings.Contains(clean, "<script") {
		t.Error("<script> not removed")
	}
	if strings.Contains(clean, "onclick") {
		t.Error("onclick not removed")
	}
	if strings.Contains(clean, "javascript:") {
		t.Error("javascript: not neutralized")
	}
	if !strings.Contains(clean, "<h1>ok</h1>") {
		t.Error("safe content lost")
	}
}

func TestWordCount(t *testing.T) {
	got := wordCount("<h1>one two three</h1><p>four five</p>")
	if got != 5 {
		t.Fatalf("got %d, want 5", got)
	}
}

func TestBuildFAQJSONLD(t *testing.T) {
	faqs := []struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}{
		{Question: "Q1", Answer: "A1"},
		{Question: "Q2", Answer: "A2"},
	}
	out := buildFAQJSONLD(faqs)
	if !strings.Contains(out, `"@type": "FAQPage"`) {
		t.Fatalf("missing FAQPage type: %s", out)
	}
	if !strings.Contains(out, `"name": "Q1"`) || !strings.Contains(out, `"name": "Q2"`) {
		t.Fatalf("missing questions: %s", out)
	}
}

func TestReplaceFirstOutsideAnchors(t *testing.T) {
	body := `<p>Learn about <a href="/foo/">foo</a> and bar in detail.</p>`
	out := replaceFirstOutsideAnchors(body, "bar", `<a href="/bar/">bar</a>`)
	if !strings.Contains(out, `<a href="/bar/">bar</a>`) {
		t.Fatalf("replacement missing: %s", out)
	}
	// Shouldn't touch the original foo anchor.
	if strings.Count(out, "/foo/") != 1 {
		t.Fatalf("foo anchor altered: %s", out)
	}
}

func TestRewriteAffiliateLinks(t *testing.T) {
	body := `<p>Buy <a href="AFFILIATE:amazon:B08X">this</a> today.</p>`
	got := rewriteAffiliateLinks(body, "mysite", "my-article")
	if !strings.Contains(got, `/go/amazon/B08X/`) {
		t.Fatalf("expected /go/amazon/B08X/ path, got: %s", got)
	}
	if !strings.Contains(got, `rel="nofollow sponsored"`) {
		t.Fatalf("expected rel=nofollow sponsored, got: %s", got)
	}
	if !strings.Contains(got, `utm_source=mysite`) || !strings.Contains(got, `utm_campaign=my-article`) {
		t.Fatalf("missing UTM params: %s", got)
	}
}

func TestMergeStrings(t *testing.T) {
	got := mergeStrings([]string{"a", "B"}, []string{"a", "c", "b"})
	if len(got) != 3 {
		t.Fatalf("expected 3 unique, got %v", got)
	}
}
