// Package seo handles Search Console analytics ingestion and the self-healing
// content optimizer loop.
package seo

import (
	"context"
	"fmt"
	"time"

	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/logging"
)

// Analytics holds SEO analytics state and a Search Console client.
type Analytics struct {
	db  *db.DB
	log *logging.Logger
	gsc *GSCClient
}

// NewAnalytics constructs an analytics engine.
func NewAnalytics(database *db.DB, log *logging.Logger, gsc *GSCClient) *Analytics {
	return &Analytics{db: database, log: log, gsc: gsc}
}

// IngestPayload is the shape accepted from Cowork (or from direct GSC calls)
// when pushing per-article metrics. Cowork typically uploads this format after
// pulling from Search Console UI.
type IngestPayload struct {
	Rows []struct {
		ArticleID   string  `json:"article_id,omitempty"`
		ArticleSlug string  `json:"article_slug,omitempty"`
		SiteID      string  `json:"site_id,omitempty"`
		SiteSlug    string  `json:"site_slug,omitempty"`
		Date        string  `json:"date"` // YYYY-MM-DD
		Impressions int     `json:"impressions"`
		Clicks      int     `json:"clicks"`
		CTR         float64 `json:"ctr"`
		AvgPosition float64 `json:"avg_position"`
		TopQuery    string  `json:"top_query"`
	} `json:"rows"`
}

// Ingest records all rows. Resolves article by slug if id absent.
func (a *Analytics) Ingest(ctx context.Context, p IngestPayload) (int, error) {
	n := 0
	for _, r := range p.Rows {
		articleID := r.ArticleID
		if articleID == "" {
			if r.ArticleSlug == "" || (r.SiteID == "" && r.SiteSlug == "") {
				continue
			}
			// Resolve site first.
			var site *db.Site
			var err error
			if r.SiteID != "" {
				site, err = a.db.GetSiteByID(ctx, r.SiteID)
			} else {
				site, err = a.db.GetSiteBySlug(ctx, r.SiteSlug)
			}
			if err != nil || site == nil {
				continue
			}
			art, err := a.db.GetArticleBySlug(ctx, site.ID, r.ArticleSlug)
			if err != nil || art == nil {
				continue
			}
			articleID = art.ID
		}
		date, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			continue
		}
		if err := a.db.UpsertAnalytics(ctx, &db.AnalyticsRow{
			ArticleID:   articleID,
			Date:        date,
			Impressions: r.Impressions,
			Clicks:      r.Clicks,
			CTR:         r.CTR,
			AvgPosition: r.AvgPosition,
			TopQuery:    r.TopQuery,
		}); err != nil {
			a.log.Warn(ctx, "analytics: upsert failed", map[string]any{"err": err.Error(), "article_id": articleID})
			continue
		}
		n++
	}
	return n, nil
}

// PullFromGSC pulls the last N days of data from Search Console for every live site.
// No-op if no GSC client is configured.
func (a *Analytics) PullFromGSC(ctx context.Context, days int) (int, error) {
	if a.gsc == nil || !a.gsc.Configured() {
		return 0, fmt.Errorf("GSC not configured (GSC_CREDENTIALS_JSON missing)")
	}
	sites, err := a.db.ListSites(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, s := range sites {
		if s.Domain == "" || s.Status != "live" {
			continue
		}
		rows, err := a.gsc.FetchSiteMetrics(ctx, s.Domain, days)
		if err != nil {
			a.log.Warn(ctx, "gsc: fetch failed", map[string]any{"site": s.Slug, "err": err.Error()})
			continue
		}
		for _, row := range rows {
			// Map row.Page (full URL) to an article slug.
			slug := extractSlug(row.Page, s.Domain)
			if slug == "" {
				continue
			}
			art, err := a.db.GetArticleBySlug(ctx, s.ID, slug)
			if err != nil || art == nil {
				continue
			}
			date, err := time.Parse("2006-01-02", row.Date)
			if err != nil {
				continue
			}
			if err := a.db.UpsertAnalytics(ctx, &db.AnalyticsRow{
				ArticleID:   art.ID,
				Date:        date,
				Impressions: row.Impressions,
				Clicks:      row.Clicks,
				CTR:         row.CTR,
				AvgPosition: row.AvgPosition,
				TopQuery:    row.Query,
			}); err == nil {
				total++
			}
		}
	}
	return total, nil
}

// Summary is the analytics summary response.
type Summary struct {
	StrikingDistance []*db.ArticleMetrics `json:"striking_distance"`
	LowCTR           []*db.ArticleMetrics `json:"low_ctr"`
	Declining        []*db.ArticleMetrics `json:"declining"`
	Cannibalization  []Cannibalization    `json:"cannibalization"`
}

// Cannibalization is when multiple articles on the same site rank for the same query.
type Cannibalization struct {
	SiteID     string   `json:"site_id"`
	Query      string   `json:"query"`
	ArticleIDs []string `json:"article_ids"`
}

// BuildSummary aggregates all the analytics signals the optimizer needs.
func (a *Analytics) BuildSummary(ctx context.Context) (*Summary, error) {
	sd, err := a.db.StrikingDistanceArticles(ctx, 28)
	if err != nil {
		return nil, err
	}
	lc, err := a.db.LowCTRArticles(ctx, 28)
	if err != nil {
		return nil, err
	}
	dec, err := a.db.DecliningArticles(ctx)
	if err != nil {
		return nil, err
	}
	// Cannibalization detection: same top_query appearing in 2+ articles per site.
	cannib, err := a.detectCannibalization(ctx)
	if err != nil {
		a.log.Warn(ctx, "cannibalization: failed", map[string]any{"err": err.Error()})
	}
	return &Summary{
		StrikingDistance: sd,
		LowCTR:           lc,
		Declining:        dec,
		Cannibalization:  cannib,
	}, nil
}

func (a *Analytics) detectCannibalization(ctx context.Context) ([]Cannibalization, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT a.site_id, an.top_query, GROUP_CONCAT(an.article_id)
		FROM analytics an
		JOIN articles a ON a.id = an.article_id
		WHERE an.top_query != '' AND an.date >= date('now','-30 day')
		GROUP BY a.site_id, an.top_query
		HAVING COUNT(DISTINCT an.article_id) >= 2
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cannibalization
	for rows.Next() {
		var (
			c        Cannibalization
			articles string
		)
		if err := rows.Scan(&c.SiteID, &c.Query, &articles); err != nil {
			return nil, err
		}
		c.ArticleIDs = splitCSV(articles)
		out = append(out, c)
	}
	return out, rows.Err()
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

