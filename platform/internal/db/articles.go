package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// UpsertArticle creates or replaces an article by (site_id, slug).
func (d *DB) UpsertArticle(ctx context.Context, a *Article) error {
	if a.ID == "" || a.SiteID == "" || a.Slug == "" {
		return fmt.Errorf("article id, site_id, slug required")
	}
	secondary, _ := json.Marshal(a.SecondaryKeywords)
	affiliate, _ := json.Marshal(a.AffiliateLinks)
	internal, _ := json.Marshal(a.InternalLinks)

	now := time.Now().UTC()
	_, err := d.ExecContext(ctx, `
		INSERT INTO articles (
			id, site_id, slug, cluster, type, title, meta_description,
			focus_keyword, secondary_keywords, body_html, faq_json_ld,
			author, word_count, status, published_at, last_optimized_at,
			generation_cost_usd, generation_hash, affiliate_links_json, internal_links_json,
			created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(site_id, slug) DO UPDATE SET
			cluster = excluded.cluster,
			type = excluded.type,
			title = excluded.title,
			meta_description = excluded.meta_description,
			focus_keyword = excluded.focus_keyword,
			secondary_keywords = excluded.secondary_keywords,
			body_html = excluded.body_html,
			faq_json_ld = excluded.faq_json_ld,
			author = excluded.author,
			word_count = excluded.word_count,
			status = excluded.status,
			published_at = excluded.published_at,
			last_optimized_at = excluded.last_optimized_at,
			generation_cost_usd = excluded.generation_cost_usd,
			generation_hash = excluded.generation_hash,
			affiliate_links_json = excluded.affiliate_links_json,
			internal_links_json = excluded.internal_links_json,
			updated_at = excluded.updated_at
	`,
		a.ID, a.SiteID, a.Slug, a.Cluster, fallback(a.Type, "article"),
		a.Title, a.MetaDescription, a.FocusKeyword, string(secondary),
		a.BodyHTML, a.FAQJSONLD, fallback(a.Author, "Editorial Team"),
		a.WordCount, fallback(a.Status, "draft"), a.PublishedAt, a.LastOptimizedAt,
		a.GenerationCostUSD, a.GenerationHash, string(affiliate), string(internal),
		now, now,
	)
	return err
}

// GetArticle returns an article by id.
func (d *DB) GetArticle(ctx context.Context, id string) (*Article, error) {
	row := d.QueryRowContext(ctx, articleSelectSQL+" WHERE id = ?", id)
	return scanArticle(row)
}

// GetArticleBySlug returns by site + slug.
func (d *DB) GetArticleBySlug(ctx context.Context, siteID, slug string) (*Article, error) {
	row := d.QueryRowContext(ctx, articleSelectSQL+" WHERE site_id = ? AND slug = ?", siteID, slug)
	return scanArticle(row)
}

// ListArticlesBySite returns all articles for a site, most recent first.
func (d *DB) ListArticlesBySite(ctx context.Context, siteID string) ([]*Article, error) {
	rows, err := d.QueryContext(ctx, articleSelectSQL+" WHERE site_id = ? ORDER BY COALESCE(published_at, created_at) DESC", siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArticleRows(rows)
}

// ListPublishedArticles returns all published articles (across all sites).
func (d *DB) ListPublishedArticles(ctx context.Context) ([]*Article, error) {
	rows, err := d.QueryContext(ctx, articleSelectSQL+" WHERE status = 'published' ORDER BY published_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArticleRows(rows)
}

// GetArticleByHash returns an article matching the generation hash (idempotency).
func (d *DB) GetArticleByHash(ctx context.Context, hash string) (*Article, error) {
	row := d.QueryRowContext(ctx, articleSelectSQL+" WHERE generation_hash = ? LIMIT 1", hash)
	return scanArticle(row)
}

// UpdateArticleStatus updates an article's status and published_at.
func (d *DB) UpdateArticleStatus(ctx context.Context, id, status string, publishedAt *time.Time) error {
	_, err := d.ExecContext(ctx, `UPDATE articles SET status = ?, published_at = COALESCE(?, published_at), updated_at = ? WHERE id = ?`, status, publishedAt, time.Now().UTC(), id)
	return err
}

// MarkOptimized sets last_optimized_at on an article.
func (d *DB) MarkOptimized(ctx context.Context, id string, at time.Time) error {
	_, err := d.ExecContext(ctx, `UPDATE articles SET last_optimized_at = ?, updated_at = ? WHERE id = ?`, at, time.Now().UTC(), id)
	return err
}

const articleSelectSQL = `SELECT
	id, site_id, slug, COALESCE(cluster,''), type, title, meta_description,
	focus_keyword, COALESCE(secondary_keywords,'[]'), body_html, COALESCE(faq_json_ld,''),
	author, word_count, status, published_at, last_optimized_at,
	generation_cost_usd, COALESCE(generation_hash,''),
	COALESCE(affiliate_links_json,'[]'), COALESCE(internal_links_json,'[]'),
	created_at, updated_at
FROM articles`

func scanArticle(r rowScanner) (*Article, error) {
	var (
		a           Article
		secondary   string
		affiliate   string
		internal    string
		published   sql.NullTime
		optimized   sql.NullTime
	)
	err := r.Scan(&a.ID, &a.SiteID, &a.Slug, &a.Cluster, &a.Type, &a.Title, &a.MetaDescription,
		&a.FocusKeyword, &secondary, &a.BodyHTML, &a.FAQJSONLD,
		&a.Author, &a.WordCount, &a.Status, &published, &optimized,
		&a.GenerationCostUSD, &a.GenerationHash, &affiliate, &internal,
		&a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if published.Valid {
		t := published.Time
		a.PublishedAt = &t
	}
	if optimized.Valid {
		t := optimized.Time
		a.LastOptimizedAt = &t
	}
	_ = json.Unmarshal([]byte(secondary), &a.SecondaryKeywords)
	_ = json.Unmarshal([]byte(affiliate), &a.AffiliateLinks)
	_ = json.Unmarshal([]byte(internal), &a.InternalLinks)
	return &a, nil
}

func scanArticleRows(rows *sql.Rows) ([]*Article, error) {
	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
