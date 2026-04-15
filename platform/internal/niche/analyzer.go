// Package niche scores the profitability of a content niche before committing
// engineering + Claude spend to it.
package niche

import (
	"context"
	"fmt"
	"strings"

	"github.com/american-desi/platform/internal/claude"
	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/generator"
	"github.com/american-desi/platform/internal/logging"
)

// Analyzer scores niches.
type Analyzer struct {
	db     *db.DB
	claude *claude.Client
	log    *logging.Logger
}

// New returns an Analyzer.
func New(database *db.DB, c *claude.Client, log *logging.Logger) *Analyzer {
	return &Analyzer{db: database, claude: c, log: log}
}

// Request bundles analyzer inputs.
type Request struct {
	Niche        string   `json:"niche"`
	SeedKeywords []string `json:"seed_keywords,omitempty"`
	Force        bool     `json:"force,omitempty"` // bypass cache
}

// Analyze returns a fresh or cached score.
func (a *Analyzer) Analyze(ctx context.Context, req Request) (*db.NicheAnalysis, error) {
	if req.Niche == "" {
		return nil, fmt.Errorf("niche required")
	}
	if !req.Force {
		if cached, err := a.db.GetNiche(ctx, req.Niche); err == nil && cached != nil {
			return cached, nil
		}
	}
	prompt := "Niche: " + req.Niche + "\n" +
		"Seed keywords: " + strings.Join(req.SeedKeywords, ", ") + "\n\n" +
		`Evaluate this niche for a new content site. Provide realistic, conservative estimates.

Output JSON:
{
  "monthly_search_vol": 0,
  "competition_level": "low|medium|high",
  "monetization_paths": ["affiliate","ads","digital_products","lead_gen"],
  "avg_affiliate_comm": 0.0,
  "est_rpm": 0.0,
  "content_velocity": 0,
  "time_to_revenue": "X months",
  "score": 0.0,
  "rationale": "2-3 sentences explaining the score"
}

Notes for estimation:
- monthly_search_vol: aggregate long-tail volume (broad, realistic)
- avg_affiliate_comm: percent commission OR dollar CPA (use percent for product sales)
- est_rpm: display ad RPM in USD per 1000 pageviews
- content_velocity: number of high-quality articles to reach topical authority
- score: 0-10 composite reflecting commission, competition, audience breadth`

	var out struct {
		MonthlySearchVol  int      `json:"monthly_search_vol"`
		CompetitionLevel  string   `json:"competition_level"`
		MonetizationPaths []string `json:"monetization_paths"`
		AvgAffiliateComm  float64  `json:"avg_affiliate_comm"`
		EstRPM            float64  `json:"est_rpm"`
		ContentVelocity   int      `json:"content_velocity"`
		TimeToRevenue     string   `json:"time_to_revenue"`
		Score             float64  `json:"score"`
		Rationale         string   `json:"rationale"`
	}
	_, err := a.claude.CompleteJSON(ctx, prompt, claude.CompleteOpts{
		Purpose:     "niche",
		System:      generator.NicheAnalysisSystemPrompt,
		Temperature: 0.3,
		MaxTokens:   1500,
	}, &out)
	if err != nil {
		return nil, err
	}

	res := &db.NicheAnalysis{
		Niche:             req.Niche,
		MonthlySearchVol:  out.MonthlySearchVol,
		CompetitionLevel:  defaultString(out.CompetitionLevel, "medium"),
		MonetizationPaths: out.MonetizationPaths,
		AvgAffiliateComm:  out.AvgAffiliateComm,
		EstRPM:            out.EstRPM,
		ContentVelocity:   out.ContentVelocity,
		TimeToRevenue:     out.TimeToRevenue,
		Score:             out.Score,
		Rationale:         out.Rationale,
	}
	if err := a.db.UpsertNiche(ctx, res); err != nil {
		return nil, err
	}
	a.log.Info(ctx, "niche: analyzed", map[string]any{
		"niche": req.Niche,
		"score": res.Score,
	})
	return res, nil
}

func defaultString(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
