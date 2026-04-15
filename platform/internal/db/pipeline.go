package db

import (
	"context"
	"database/sql"
	"time"
)

// EnqueuePipelineItem adds an article generation task to the queue.
func (d *DB) EnqueuePipelineItem(ctx context.Context, p *PipelineItem) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO pipeline (site_id, focus_keyword, cluster, article_type, scheduled_for, status)
		VALUES (?,?,?,?,?,?)
	`, p.SiteID, p.FocusKeyword, p.Cluster, fallback(p.ArticleType, "article"), p.ScheduledFor, fallback(p.Status, "queued"))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DuePipelineItems returns queued items whose scheduled_for <= now.
func (d *DB) DuePipelineItems(ctx context.Context, limit int) ([]*PipelineItem, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := d.QueryContext(ctx, `
		SELECT id, site_id, focus_keyword, COALESCE(cluster,''), article_type, scheduled_for, status, COALESCE(article_id,''), attempts, COALESCE(last_error,''), created_at, updated_at
		FROM pipeline
		WHERE status = 'queued' AND scheduled_for <= ?
		ORDER BY scheduled_for ASC
		LIMIT ?
	`, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPipelineRows(rows)
}

// ListPipelineItems lists recent queue entries.
func (d *DB) ListPipelineItems(ctx context.Context, limit int) ([]*PipelineItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.QueryContext(ctx, `
		SELECT id, site_id, focus_keyword, COALESCE(cluster,''), article_type, scheduled_for, status, COALESCE(article_id,''), attempts, COALESCE(last_error,''), created_at, updated_at
		FROM pipeline
		ORDER BY scheduled_for DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPipelineRows(rows)
}

// UpdatePipelineStatus updates status + attempt metadata on a pipeline item.
func (d *DB) UpdatePipelineStatus(ctx context.Context, id int64, status, articleID, lastErr string) error {
	_, err := d.ExecContext(ctx, `
		UPDATE pipeline SET
			status = ?,
			article_id = COALESCE(NULLIF(?,''), article_id),
			attempts = attempts + ?,
			last_error = ?,
			updated_at = ?
		WHERE id = ?
	`, status, articleID, attemptIncrement(status), lastErr, time.Now().UTC(), id)
	return err
}

func attemptIncrement(status string) int {
	if status == "generating" || status == "failed" {
		return 1
	}
	return 0
}

func scanPipelineRows(rows *sql.Rows) ([]*PipelineItem, error) {
	var out []*PipelineItem
	for rows.Next() {
		var p PipelineItem
		if err := rows.Scan(&p.ID, &p.SiteID, &p.FocusKeyword, &p.Cluster, &p.ArticleType, &p.ScheduledFor, &p.Status, &p.ArticleID, &p.Attempts, &p.LastError, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}
