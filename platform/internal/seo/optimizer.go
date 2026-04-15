package seo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/american-desi/platform/internal/claude"
	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/generator"
	"github.com/american-desi/platform/internal/logging"
)

// Optimizer implements the self-healing content loop.
type Optimizer struct {
	db       *db.DB
	claude   *claude.Client
	log      *logging.Logger
	minAgeDays int
}

// NewOptimizer constructs an optimizer.
func NewOptimizer(database *db.DB, cc *claude.Client, log *logging.Logger, minAgeDays int) *Optimizer {
	if minAgeDays <= 0 {
		minAgeDays = 60
	}
	return &Optimizer{db: database, claude: cc, log: log, minAgeDays: minAgeDays}
}

// RunResult summarizes one optimization cycle.
type RunResult struct {
	Timestamp        time.Time `json:"timestamp"`
	Rewrites         int       `json:"rewrites"`
	MetaRefreshes    int       `json:"meta_refreshes"`
	ContentRefreshes int       `json:"content_refreshes"`
	Skipped          int       `json:"skipped"`
	Errors           int       `json:"errors"`
	CostUSD          float64   `json:"cost_usd"`
}

// Run performs one full optimization cycle. Force bypasses the minimum-age gate.
func (o *Optimizer) Run(ctx context.Context, force bool) (*RunResult, error) {
	res := &RunResult{Timestamp: time.Now().UTC()}
	a := NewAnalytics(o.db, o.log, nil)
	summary, err := a.BuildSummary(ctx)
	if err != nil {
		return res, err
	}

	// 1. Striking distance: rewrite for depth.
	for _, m := range summary.StrikingDistance {
		art, err := o.db.GetArticle(ctx, m.ArticleID)
		if err != nil || art == nil {
			continue
		}
		if !force && !o.eligibleByAge(art) {
			res.Skipped++
			continue
		}
		cost, err := o.rewriteArticle(ctx, art, m, "striking_distance")
		if err != nil {
			o.log.Warn(ctx, "optimizer: rewrite failed", map[string]any{"article_id": art.ID, "err": err.Error()})
			res.Errors++
			continue
		}
		res.CostUSD += cost
		res.Rewrites++
	}

	// 2. Low CTR: rewrite title + meta.
	for _, m := range summary.LowCTR {
		art, err := o.db.GetArticle(ctx, m.ArticleID)
		if err != nil || art == nil {
			continue
		}
		if !force && !o.eligibleByAge(art) {
			res.Skipped++
			continue
		}
		cost, err := o.rewriteMeta(ctx, art, m)
		if err != nil {
			o.log.Warn(ctx, "optimizer: meta rewrite failed", map[string]any{"article_id": art.ID, "err": err.Error()})
			res.Errors++
			continue
		}
		res.CostUSD += cost
		res.MetaRefreshes++
	}

	// 3. Declining: content refresh (add new sections / stats).
	for _, m := range summary.Declining {
		art, err := o.db.GetArticle(ctx, m.ArticleID)
		if err != nil || art == nil {
			continue
		}
		if !force && !o.eligibleByAge(art) {
			res.Skipped++
			continue
		}
		cost, err := o.refreshContent(ctx, art, m)
		if err != nil {
			o.log.Warn(ctx, "optimizer: refresh failed", map[string]any{"article_id": art.ID, "err": err.Error()})
			res.Errors++
			continue
		}
		res.CostUSD += cost
		res.ContentRefreshes++
	}

	o.log.Info(ctx, "optimizer: cycle complete", map[string]any{
		"rewrites":          res.Rewrites,
		"meta_refreshes":    res.MetaRefreshes,
		"content_refreshes": res.ContentRefreshes,
		"skipped":           res.Skipped,
		"errors":            res.Errors,
		"cost_usd":          res.CostUSD,
	})
	return res, nil
}

func (o *Optimizer) eligibleByAge(a *db.Article) bool {
	ref := a.CreatedAt
	if a.PublishedAt != nil {
		ref = *a.PublishedAt
	}
	return time.Since(ref) >= time.Duration(o.minAgeDays)*24*time.Hour
}

func (o *Optimizer) rewriteArticle(ctx context.Context, art *db.Article, m *db.ArticleMetrics, reason string) (float64, error) {
	before := snapshot(art)
	beforeMetrics := metricsJSON(m)

	prompt := fmt.Sprintf(`Current article HTML:
%s

Current metrics (last 28d): %d impressions, %d clicks, avg position %.1f.
Top query: %q.

Task:
- Rewrite this article to rank higher for its focus keyword "%s".
- Add 300-600 words of fresh depth: new examples, updated statistics, expert perspectives, comparison tables where appropriate.
- Tighten any weak/generic sections.
- Preserve all existing AFFILIATE: placeholders and internal links.
- Keep the same semantic HTML structure (h1, h2, h3, p, ul, ol, table).
- Do not change the slug or focus keyword.

Output JSON:
{
  "title": "(may stay the same or improve)",
  "meta_description": "(may stay the same or improve)",
  "body_html": "<h1>...</h1>...",
  "changelog": "one-line summary of what you changed"
}`,
		art.BodyHTML, m.Impressions, m.Clicks, m.AvgPosition, m.TopQuery, art.FocusKeyword)

	var payload struct {
		Title           string `json:"title"`
		MetaDescription string `json:"meta_description"`
		BodyHTML        string `json:"body_html"`
		Changelog       string `json:"changelog"`
	}
	resp, err := o.claude.CompleteJSON(ctx, prompt, claude.CompleteOpts{
		Purpose:     "optimize",
		System:      generator.OptimizationSystemPrompt,
		Temperature: 0.5,
		MaxTokens:   8000,
	}, &payload)
	if err != nil {
		return 0, err
	}
	cost := claude.CostUSD(resp.Usage.InputTokens, resp.Usage.OutputTokens)

	if strings.TrimSpace(payload.BodyHTML) == "" {
		return cost, fmt.Errorf("empty body returned")
	}
	if payload.Title != "" {
		art.Title = payload.Title
	}
	if payload.MetaDescription != "" {
		art.MetaDescription = payload.MetaDescription
	}
	art.BodyHTML = payload.BodyHTML
	now := time.Now().UTC()
	art.LastOptimizedAt = &now
	if err := o.db.UpsertArticle(ctx, art); err != nil {
		return cost, err
	}
	_ = o.db.MarkOptimized(ctx, art.ID, now)
	_ = o.db.InsertOptimization(ctx, &db.Optimization{
		ArticleID:      art.ID,
		Kind:           "rewrite",
		Reason:         reason,
		BeforeSnapshot: before,
		AfterSnapshot:  snapshot(art) + "\n\nCHANGELOG: " + payload.Changelog,
		BeforeMetrics:  beforeMetrics,
		CostUSD:        cost,
	})
	return cost, nil
}

func (o *Optimizer) rewriteMeta(ctx context.Context, art *db.Article, m *db.ArticleMetrics) (float64, error) {
	before := snapshot(art)
	beforeMetrics := metricsJSON(m)

	prompt := fmt.Sprintf(`Focus keyword: %s
Article topic summary: %s

Current title: %q (%d chars)
Current meta description: %q (%d chars)
Current CTR: %.2f%% at avg position %.1f

Task: Produce 3 alternative (title, meta_description) pairs that will increase CTR. Then pick the best one.

Output JSON:
{"alternatives":[{"title":"...","meta_description":"...","rationale":"..."}, {...}, {...}], "best_index": 0}`,
		art.FocusKeyword, art.MetaDescription, art.Title, len(art.Title), art.MetaDescription, len(art.MetaDescription), m.CTR*100, m.AvgPosition)

	var payload struct {
		Alternatives []struct {
			Title           string `json:"title"`
			MetaDescription string `json:"meta_description"`
			Rationale       string `json:"rationale"`
		} `json:"alternatives"`
		BestIndex int `json:"best_index"`
	}
	resp, err := o.claude.CompleteJSON(ctx, prompt, claude.CompleteOpts{
		Purpose:     "meta_rewrite",
		System:      generator.MetaRewriteSystemPrompt,
		Temperature: 0.8,
		MaxTokens:   1500,
	}, &payload)
	if err != nil {
		return 0, err
	}
	cost := claude.CostUSD(resp.Usage.InputTokens, resp.Usage.OutputTokens)
	if len(payload.Alternatives) == 0 {
		return cost, fmt.Errorf("no alternatives returned")
	}
	idx := payload.BestIndex
	if idx < 0 || idx >= len(payload.Alternatives) {
		idx = 0
	}
	best := payload.Alternatives[idx]
	if best.Title != "" {
		art.Title = best.Title
	}
	if best.MetaDescription != "" {
		art.MetaDescription = best.MetaDescription
	}
	now := time.Now().UTC()
	art.LastOptimizedAt = &now
	if err := o.db.UpsertArticle(ctx, art); err != nil {
		return cost, err
	}
	_ = o.db.InsertOptimization(ctx, &db.Optimization{
		ArticleID:      art.ID,
		Kind:           "meta_refresh",
		Reason:         "low_ctr",
		BeforeSnapshot: before,
		AfterSnapshot:  snapshot(art) + "\n\nRATIONALE: " + best.Rationale,
		BeforeMetrics:  beforeMetrics,
		CostUSD:        cost,
	})
	return cost, nil
}

func (o *Optimizer) refreshContent(ctx context.Context, art *db.Article, m *db.ArticleMetrics) (float64, error) {
	before := snapshot(art)
	beforeMetrics := metricsJSON(m)

	prompt := fmt.Sprintf(`This article has been declining in organic impressions (top query %q, avg position %.1f). Refresh it for topical freshness.

Current HTML:
%s

Task:
- Add 1-2 new H2 sections covering gaps a reader would expect in %d — new trends, fresh data, current best-of examples.
- Update any stats/years in the article.
- Append 2-3 new FAQ entries addressing "People Also Ask"-style questions.
- Preserve all AFFILIATE: placeholders and internal links.
- Do not change the slug.

Output JSON:
{
  "body_html": "<h1>...</h1>...",
  "added_sections": ["H2 heading 1", "H2 heading 2"],
  "new_faq": [{"question":"...","answer":"..."}]
}`, m.TopQuery, m.AvgPosition, art.BodyHTML, time.Now().Year())

	var payload struct {
		BodyHTML      string `json:"body_html"`
		AddedSections []string `json:"added_sections"`
		NewFAQ        []struct {
			Question string `json:"question"`
			Answer   string `json:"answer"`
		} `json:"new_faq"`
	}
	resp, err := o.claude.CompleteJSON(ctx, prompt, claude.CompleteOpts{
		Purpose:     "content_refresh",
		System:      generator.OptimizationSystemPrompt,
		Temperature: 0.6,
		MaxTokens:   8000,
	}, &payload)
	if err != nil {
		return 0, err
	}
	cost := claude.CostUSD(resp.Usage.InputTokens, resp.Usage.OutputTokens)
	if strings.TrimSpace(payload.BodyHTML) == "" {
		return cost, fmt.Errorf("empty body returned")
	}
	art.BodyHTML = payload.BodyHTML
	now := time.Now().UTC()
	art.LastOptimizedAt = &now
	if err := o.db.UpsertArticle(ctx, art); err != nil {
		return cost, err
	}
	_ = o.db.InsertOptimization(ctx, &db.Optimization{
		ArticleID:      art.ID,
		Kind:           "content_refresh",
		Reason:         "declining",
		BeforeSnapshot: before,
		AfterSnapshot:  snapshot(art) + "\n\nADDED: " + strings.Join(payload.AddedSections, ", "),
		BeforeMetrics:  beforeMetrics,
		CostUSD:        cost,
	})
	return cost, nil
}

// Loop runs the optimizer on a ticker.
func (o *Optimizer) Loop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 7 * 24 * time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			func() {
				runCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
				defer cancel()
				runCtx = logging.WithRequestID(runCtx, "optimizer-"+time.Now().UTC().Format("20060102-150405"))
				if _, err := o.Run(runCtx, false); err != nil {
					o.log.Error(runCtx, "optimizer: cycle failed", map[string]any{"err": err.Error()})
				}
			}()
		}
	}
}

func snapshot(a *db.Article) string {
	b, _ := json.Marshal(map[string]any{
		"title":            a.Title,
		"meta_description": a.MetaDescription,
		"word_count":       a.WordCount,
		"updated_at":       a.UpdatedAt,
	})
	return string(b)
}

func metricsJSON(m *db.ArticleMetrics) string {
	b, _ := json.Marshal(m)
	return string(b)
}
