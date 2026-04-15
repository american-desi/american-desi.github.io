package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// InsertSite persists a new site. Returns the created row with timestamps filled in.
func (d *DB) InsertSite(ctx context.Context, s *Site) error {
	if s.ID == "" || s.Slug == "" || s.Niche == "" {
		return fmt.Errorf("site id, slug, niche are required")
	}
	seeds, _ := json.Marshal(s.SeedKeywords)
	progs, _ := json.Marshal(s.AffiliateProgram)
	_, err := d.ExecContext(ctx, `
		INSERT INTO sites (id, slug, domain, niche, tagline, description, seed_keywords, affiliate_program, ad_network, ads_txt, status, health_score)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.Slug, nullIfEmpty(s.Domain), s.Niche, s.Tagline, s.Description, string(seeds), string(progs), s.AdNetwork, s.AdsTxt, fallback(s.Status, "draft"), fallbackF(s.HealthScore, 1.0))
	return err
}

// GetSiteBySlug fetches a site by slug or returns sql.ErrNoRows.
func (d *DB) GetSiteBySlug(ctx context.Context, slug string) (*Site, error) {
	row := d.QueryRowContext(ctx, `SELECT id, slug, COALESCE(domain,''), niche, COALESCE(tagline,''), COALESCE(description,''), COALESCE(seed_keywords,'[]'), COALESCE(affiliate_program,'[]'), COALESCE(ad_network,''), COALESCE(ads_txt,''), status, health_score, created_at, updated_at FROM sites WHERE slug = ?`, slug)
	return scanSite(row)
}

// GetSiteByID fetches a site by id.
func (d *DB) GetSiteByID(ctx context.Context, id string) (*Site, error) {
	row := d.QueryRowContext(ctx, `SELECT id, slug, COALESCE(domain,''), niche, COALESCE(tagline,''), COALESCE(description,''), COALESCE(seed_keywords,'[]'), COALESCE(affiliate_program,'[]'), COALESCE(ad_network,''), COALESCE(ads_txt,''), status, health_score, created_at, updated_at FROM sites WHERE id = ?`, id)
	return scanSite(row)
}

// ListSites returns all sites ordered by most recent first.
func (d *DB) ListSites(ctx context.Context) ([]*Site, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, slug, COALESCE(domain,''), niche, COALESCE(tagline,''), COALESCE(description,''), COALESCE(seed_keywords,'[]'), COALESCE(affiliate_program,'[]'), COALESCE(ad_network,''), COALESCE(ads_txt,''), status, health_score, created_at, updated_at FROM sites ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Site
	for rows.Next() {
		s, err := scanSiteRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateSite updates mutable fields.
func (d *DB) UpdateSite(ctx context.Context, s *Site) error {
	seeds, _ := json.Marshal(s.SeedKeywords)
	progs, _ := json.Marshal(s.AffiliateProgram)
	_, err := d.ExecContext(ctx, `
		UPDATE sites SET
			domain = ?, niche = ?, tagline = ?, description = ?,
			seed_keywords = ?, affiliate_program = ?, ad_network = ?, ads_txt = ?,
			status = ?, health_score = ?, updated_at = ?
		WHERE id = ?
	`, nullIfEmpty(s.Domain), s.Niche, s.Tagline, s.Description, string(seeds), string(progs), s.AdNetwork, s.AdsTxt, s.Status, s.HealthScore, time.Now().UTC(), s.ID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSite(r rowScanner) (*Site, error) {
	var (
		s                 Site
		seedJSON, progJSON string
	)
	err := r.Scan(&s.ID, &s.Slug, &s.Domain, &s.Niche, &s.Tagline, &s.Description, &seedJSON, &progJSON, &s.AdNetwork, &s.AdsTxt, &s.Status, &s.HealthScore, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(seedJSON), &s.SeedKeywords)
	_ = json.Unmarshal([]byte(progJSON), &s.AffiliateProgram)
	return &s, nil
}

func scanSiteRows(r *sql.Rows) (*Site, error) { return scanSite(r) }

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func fallbackF(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}
