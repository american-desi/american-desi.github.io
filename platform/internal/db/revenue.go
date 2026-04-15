package db

import "context"

// UpsertRevenue records revenue for a site/date/source tuple.
func (d *DB) UpsertRevenue(ctx context.Context, r *RevenueRow) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO revenue (site_id, date, source, revenue_usd, clicks, conversions, sessions, raw_payload)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(site_id, date, source) DO UPDATE SET
			revenue_usd = excluded.revenue_usd,
			clicks = excluded.clicks,
			conversions = excluded.conversions,
			sessions = excluded.sessions,
			raw_payload = excluded.raw_payload
	`, r.SiteID, r.Date.Format("2006-01-02"), r.Source, r.RevenueUSD, r.Clicks, r.Conversions, r.Sessions, r.RawPayload)
	return err
}

// RevenueSummary rolls up total revenue per site over the last N days.
type SiteRevenueSummary struct {
	SiteID      string  `json:"site_id"`
	SiteSlug    string  `json:"site_slug"`
	RevenueUSD  float64 `json:"revenue_usd"`
	Clicks      int     `json:"clicks"`
	Conversions int     `json:"conversions"`
	Sessions    int     `json:"sessions"`
}

// RevenueSummary returns rollups per site for the last N days.
func (d *DB) RevenueSummary(ctx context.Context, days int) ([]*SiteRevenueSummary, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT s.id, s.slug, COALESCE(SUM(r.revenue_usd),0), COALESCE(SUM(r.clicks),0), COALESCE(SUM(r.conversions),0), COALESCE(SUM(r.sessions),0)
		FROM sites s
		LEFT JOIN revenue r ON r.site_id = s.id AND r.date >= date('now', ?)
		GROUP BY s.id
		ORDER BY 3 DESC
	`, "-"+intStr(days)+" day")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*SiteRevenueSummary
	for rows.Next() {
		var r SiteRevenueSummary
		if err := rows.Scan(&r.SiteID, &r.SiteSlug, &r.RevenueUSD, &r.Clicks, &r.Conversions, &r.Sessions); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

func intStr(n int) string {
	if n < 0 {
		n = -n
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
