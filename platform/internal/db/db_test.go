package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestMigrationsApplied(t *testing.T) {
	d := testDB(t)
	var n int
	if err := d.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 migration applied, got %d", n)
	}
	// Seed niches migration should have populated rows.
	if err := d.QueryRow(`SELECT COUNT(*) FROM niche_analyses`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 10 {
		t.Fatalf("expected 10 seed niches, got %d", n)
	}
}

func TestSiteCRUD(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	s := &Site{
		ID:               uuid.NewString(),
		Slug:             "test-site",
		Niche:            "test-niche",
		Tagline:          "tagline",
		SeedKeywords:     []string{"a", "b"},
		AffiliateProgram: []string{"amazon"},
		AdNetwork:        "ezoic",
	}
	if err := d.InsertSite(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := d.GetSiteBySlug(ctx, "test-site")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Niche != "test-niche" || len(got.SeedKeywords) != 2 || got.AdNetwork != "ezoic" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	sites, err := d.ListSites(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(sites))
	}
}

func TestArticleUpsertIdempotent(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	site := &Site{ID: uuid.NewString(), Slug: "s", Niche: "n"}
	_ = d.InsertSite(ctx, site)
	a := &Article{
		ID:              uuid.NewString(),
		SiteID:          site.ID,
		Slug:            "my-article",
		Title:           "T",
		MetaDescription: "D",
		FocusKeyword:    "k",
		BodyHTML:        "<h1>t</h1>",
		Status:          "draft",
		GenerationHash:  "abc123",
	}
	if err := d.UpsertArticle(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := d.UpsertArticle(ctx, a); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := d.GetArticleByHash(ctx, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != a.ID {
		t.Fatal("hash lookup mismatch")
	}
}

func TestAnalyticsStrikingDistance(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	site := &Site{ID: uuid.NewString(), Slug: "s2", Niche: "n"}
	_ = d.InsertSite(ctx, site)
	a := &Article{ID: uuid.NewString(), SiteID: site.ID, Slug: "article-1", Title: "T", MetaDescription: "D", FocusKeyword: "k", BodyHTML: "<p>.</p>", Status: "published"}
	_ = d.UpsertArticle(ctx, a)
	today := time.Now().UTC()
	// Position 8 with > 50 impressions → striking distance.
	for i := 0; i < 5; i++ {
		_ = d.UpsertAnalytics(ctx, &AnalyticsRow{
			ArticleID:   a.ID,
			Date:        today.AddDate(0, 0, -i),
			Impressions: 20,
			Clicks:      1,
			AvgPosition: 8.0,
			TopQuery:    "example query",
		})
	}
	rows, err := d.StrikingDistanceArticles(ctx, 28)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ArticleID != a.ID {
		t.Fatalf("expected 1 striking-distance row for %s, got %+v", a.ID, rows)
	}
}

func TestNicheCache(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	n := &NicheAnalysis{
		Niche:             "custom-niche",
		Score:             7.5,
		MonetizationPaths: []string{"affiliate"},
	}
	if err := d.UpsertNiche(ctx, n); err != nil {
		t.Fatal(err)
	}
	got, err := d.GetNiche(ctx, "custom-niche")
	if err != nil {
		t.Fatal(err)
	}
	if got.Score != 7.5 || len(got.MonetizationPaths) != 1 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestRevenueSummary(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	site := &Site{ID: uuid.NewString(), Slug: "rev", Niche: "n"}
	_ = d.InsertSite(ctx, site)
	_ = d.UpsertRevenue(ctx, &RevenueRow{
		SiteID:     site.ID,
		Date:       time.Now().UTC(),
		Source:     "amazon",
		RevenueUSD: 12.34,
		Sessions:   100,
	})
	rows, err := d.RevenueSummary(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || rows[0].RevenueUSD != 12.34 {
		t.Fatalf("summary: %+v", rows)
	}
}
