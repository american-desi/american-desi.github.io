package db

import (
	"context"
	"database/sql"
	"time"
)

// UpsertAnalytics inserts or updates a daily analytics row.
func (d *DB) UpsertAnalytics(ctx context.Context, r *AnalyticsRow) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO analytics (article_id, date, impressions, clicks, ctr, avg_position, top_query)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(article_id, date) DO UPDATE SET
			impressions = excluded.impressions,
			clicks = excluded.clicks,
			ctr = excluded.ctr,
			avg_position = excluded.avg_position,
			top_query = excluded.top_query
	`, r.ArticleID, r.Date.Format("2006-01-02"), r.Impressions, r.Clicks, r.CTR, r.AvgPosition, r.TopQuery)
	return err
}

// ArticleMetrics aggregates metrics across a date window.
type ArticleMetrics struct {
	ArticleID   string
	Impressions int
	Clicks      int
	CTR         float64
	AvgPosition float64
	TopQuery    string
}

// MetricsForArticle returns aggregated metrics for the last N days.
func (d *DB) MetricsForArticle(ctx context.Context, articleID string, days int) (*ArticleMetrics, error) {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	row := d.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(impressions),0), COALESCE(SUM(clicks),0),
		       COALESCE(AVG(avg_position),0), COALESCE(MAX(top_query),'')
		FROM analytics WHERE article_id = ? AND date >= ?
	`, articleID, since)
	m := &ArticleMetrics{ArticleID: articleID}
	if err := row.Scan(&m.Impressions, &m.Clicks, &m.AvgPosition, &m.TopQuery); err != nil {
		return nil, err
	}
	if m.Impressions > 0 {
		m.CTR = float64(m.Clicks) / float64(m.Impressions)
	}
	return m, nil
}

// StrikingDistanceArticles returns articles averaging position 5-20 over window.
func (d *DB) StrikingDistanceArticles(ctx context.Context, days int) ([]*ArticleMetrics, error) {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := d.QueryContext(ctx, `
		SELECT article_id,
		       SUM(impressions), SUM(clicks), AVG(avg_position), MAX(top_query)
		FROM analytics
		WHERE date >= ?
		GROUP BY article_id
		HAVING AVG(avg_position) BETWEEN 5 AND 20 AND SUM(impressions) > 50
		ORDER BY SUM(impressions) DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

// LowCTRArticles: >= 500 impressions over window but CTR < 2%.
func (d *DB) LowCTRArticles(ctx context.Context, days int) ([]*ArticleMetrics, error) {
	since := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := d.QueryContext(ctx, `
		SELECT article_id,
		       SUM(impressions), SUM(clicks), AVG(avg_position), MAX(top_query)
		FROM analytics
		WHERE date >= ?
		GROUP BY article_id
		HAVING SUM(impressions) >= 500 AND (CAST(SUM(clicks) AS REAL) / SUM(impressions)) < 0.02
		ORDER BY SUM(impressions) DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

// DecliningArticles: last-14d impressions < 60% of previous-14d.
func (d *DB) DecliningArticles(ctx context.Context) ([]*ArticleMetrics, error) {
	rows, err := d.QueryContext(ctx, `
		WITH recent AS (
			SELECT article_id, SUM(impressions) AS imp, AVG(avg_position) AS pos, MAX(top_query) AS q
			FROM analytics WHERE date >= date('now','-14 day') GROUP BY article_id
		),
		prior AS (
			SELECT article_id, SUM(impressions) AS imp
			FROM analytics WHERE date >= date('now','-28 day') AND date < date('now','-14 day') GROUP BY article_id
		)
		SELECT r.article_id, r.imp, 0, r.pos, r.q
		FROM recent r JOIN prior p USING (article_id)
		WHERE p.imp > 100 AND (CAST(r.imp AS REAL) / p.imp) < 0.6
		ORDER BY p.imp DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMetrics(rows)
}

func scanMetrics(rows *sql.Rows) ([]*ArticleMetrics, error) {
	var out []*ArticleMetrics
	for rows.Next() {
		var m ArticleMetrics
		var query sql.NullString
		if err := rows.Scan(&m.ArticleID, &m.Impressions, &m.Clicks, &m.AvgPosition, &query); err != nil {
			return nil, err
		}
		if query.Valid {
			m.TopQuery = query.String
		}
		if m.Impressions > 0 {
			m.CTR = float64(m.Clicks) / float64(m.Impressions)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}
