package db

import (
	"context"
	"database/sql"
)

// UpsertKeyword inserts or updates a keyword by (site_id, keyword).
func (d *DB) UpsertKeyword(ctx context.Context, k *Keyword) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO keywords (site_id, keyword, intent, cluster, search_volume, difficulty, covered, priority)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(site_id, keyword) DO UPDATE SET
			intent = excluded.intent,
			cluster = excluded.cluster,
			search_volume = excluded.search_volume,
			difficulty = excluded.difficulty,
			covered = excluded.covered,
			priority = excluded.priority
	`, k.SiteID, k.Keyword, k.Intent, k.Cluster, k.SearchVolume, k.Difficulty, boolToInt(k.Covered), k.Priority)
	return err
}

// UncoveredKeywords returns the highest-priority uncovered keywords for a site.
func (d *DB) UncoveredKeywords(ctx context.Context, siteID string, limit int) ([]*Keyword, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := d.QueryContext(ctx, `SELECT id, site_id, keyword, intent, COALESCE(cluster,''), search_volume, difficulty, covered, priority, discovered_at FROM keywords WHERE site_id = ? AND covered = 0 ORDER BY priority ASC, search_volume DESC LIMIT ?`, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKeywords(rows)
}

// ListKeywords for a site.
func (d *DB) ListKeywords(ctx context.Context, siteID string) ([]*Keyword, error) {
	rows, err := d.QueryContext(ctx, `SELECT id, site_id, keyword, intent, COALESCE(cluster,''), search_volume, difficulty, covered, priority, discovered_at FROM keywords WHERE site_id = ? ORDER BY priority ASC, search_volume DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKeywords(rows)
}

// MarkKeywordCovered marks a keyword as covered by an article.
func (d *DB) MarkKeywordCovered(ctx context.Context, siteID, keyword string) error {
	_, err := d.ExecContext(ctx, `UPDATE keywords SET covered = 1 WHERE site_id = ? AND keyword = ?`, siteID, keyword)
	return err
}

func scanKeywords(rows *sql.Rows) ([]*Keyword, error) {
	var out []*Keyword
	for rows.Next() {
		var (
			k          Keyword
			coveredInt int
		)
		if err := rows.Scan(&k.ID, &k.SiteID, &k.Keyword, &k.Intent, &k.Cluster, &k.SearchVolume, &k.Difficulty, &coveredInt, &k.Priority, &k.DiscoveredAt); err != nil {
			return nil, err
		}
		k.Covered = coveredInt != 0
		out = append(out, &k)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
