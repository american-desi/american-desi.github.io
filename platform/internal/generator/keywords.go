package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/american-desi/platform/internal/claude"
	"github.com/american-desi/platform/internal/db"
)

// KeywordMapRequest drives keyword expansion.
type KeywordMapRequest struct {
	SiteID        string
	Niche         string
	SeedKeywords  []string
	TargetCount   int // default 30
}

type keywordPayload struct {
	Clusters []struct {
		Cluster  string `json:"cluster"`
		Keywords []struct {
			Keyword      string `json:"keyword"`
			Intent       string `json:"intent"`
			SearchVolume int    `json:"search_volume"`
			Difficulty   int    `json:"difficulty"`
			Priority     int    `json:"priority"`
		} `json:"keywords"`
	} `json:"clusters"`
}

// GenerateKeywordMap asks Claude to expand a seed list into a structured map and
// upserts all rows into the keywords table.
func (e *Engine) GenerateKeywordMap(ctx context.Context, req KeywordMapRequest) (int, error) {
	if req.SiteID == "" || req.Niche == "" {
		return 0, fmt.Errorf("site_id and niche required")
	}
	if req.TargetCount == 0 {
		req.TargetCount = 30
	}

	prompt := fmt.Sprintf(`Niche: %s
Seed keywords: %s
Produce a keyword map covering ~%d total keywords organized into 3-6 clusters. Each cluster should mix:
- informational keywords (how-to, what-is, guide)
- commercial keywords (best, review, vs, comparison)
- transactional keywords (buy, price, discount, deal)

For each keyword provide realistic estimates:
- search_volume: integer monthly searches (be conservative; most long-tail < 5000)
- difficulty: 0-100 (roughly Ahrefs KD)
- priority: 1 (high) to 10 (low), based on volume*commercial_value/difficulty
- intent: informational | commercial | transactional

Output JSON:
{"clusters":[{"cluster":"name","keywords":[{"keyword":"...","intent":"...","search_volume":0,"difficulty":0,"priority":0}]}]}`,
		req.Niche, strings.Join(req.SeedKeywords, ", "), req.TargetCount)

	var payload keywordPayload
	_, err := e.claude.CompleteJSON(ctx, prompt, claude.CompleteOpts{
		Purpose:     "keyword_map",
		System:      KeywordMapSystemPrompt,
		Temperature: 0.3,
		MaxTokens:   4000,
	}, &payload)
	if err != nil {
		return 0, fmt.Errorf("claude: %w", err)
	}

	n := 0
	for _, c := range payload.Clusters {
		for _, k := range c.Keywords {
			kw := strings.TrimSpace(k.Keyword)
			if kw == "" {
				continue
			}
			intent := k.Intent
			if intent == "" {
				intent = "informational"
			}
			err := e.db.UpsertKeyword(ctx, &db.Keyword{
				SiteID:       req.SiteID,
				Keyword:      kw,
				Intent:       intent,
				Cluster:      c.Cluster,
				SearchVolume: k.SearchVolume,
				Difficulty:   float64(k.Difficulty),
				Priority:     fallbackInt(k.Priority, 5),
			})
			if err != nil {
				return n, fmt.Errorf("upsert keyword %q: %w", kw, err)
			}
			n++
		}
	}
	return n, nil
}

func fallbackInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
