package db

import (
	"context"
	"database/sql"
)

// InsertOptimization logs a self-healing action.
func (d *DB) InsertOptimization(ctx context.Context, o *Optimization) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO optimizations (article_id, kind, reason, before_snapshot, after_snapshot, before_metrics, after_metrics, cost_usd)
		VALUES (?,?,?,?,?,?,?,?)
	`, o.ArticleID, o.Kind, o.Reason, o.BeforeSnapshot, o.AfterSnapshot, o.BeforeMetrics, nullIfEmpty(o.AfterMetrics), o.CostUSD)
	return err
}

// ListOptimizations returns recent optimizations.
func (d *DB) ListOptimizations(ctx context.Context, limit int) ([]*Optimization, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.QueryContext(ctx, `SELECT id, article_id, kind, reason, COALESCE(before_snapshot,''), COALESCE(after_snapshot,''), COALESCE(before_metrics,''), COALESCE(after_metrics,''), cost_usd, created_at FROM optimizations ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Optimization
	for rows.Next() {
		var (
			o         Optimization
			afterNull sql.NullString
		)
		if err := rows.Scan(&o.ID, &o.ArticleID, &o.Kind, &o.Reason, &o.BeforeSnapshot, &o.AfterSnapshot, &o.BeforeMetrics, &afterNull, &o.CostUSD, &o.CreatedAt); err != nil {
			return nil, err
		}
		if afterNull.Valid {
			o.AfterMetrics = afterNull.String
		}
		out = append(out, &o)
	}
	return out, rows.Err()
}
