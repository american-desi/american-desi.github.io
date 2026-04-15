package db

import (
	"context"
	"time"
)

// RecordClaudeUsage inserts a usage row.
func (d *DB) RecordClaudeUsage(ctx context.Context, u *ClaudeUsage) error {
	_, err := d.ExecContext(ctx, `
		INSERT INTO claude_usage (request_id, purpose, model, input_tokens, output_tokens, cost_usd)
		VALUES (?,?,?,?,?,?)
	`, u.RequestID, u.Purpose, u.Model, u.InputTokens, u.OutputTokens, u.CostUSD)
	return err
}

// MonthToDateSpend returns USD spent on Claude API since the 1st of this month.
func (d *DB) MonthToDateSpend(ctx context.Context) (float64, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var sum float64
	row := d.QueryRowContext(ctx, `SELECT COALESCE(SUM(cost_usd),0) FROM claude_usage WHERE created_at >= ?`, start)
	if err := row.Scan(&sum); err != nil {
		return 0, err
	}
	return sum, nil
}

// SpendByPurpose returns USD spend grouped by purpose for the last N days.
type SpendByPurpose struct {
	Purpose  string  `json:"purpose"`
	CostUSD  float64 `json:"cost_usd"`
	Requests int     `json:"requests"`
}

// ClaudeSpendByPurpose groups usage by purpose.
func (d *DB) ClaudeSpendByPurpose(ctx context.Context, days int) ([]*SpendByPurpose, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	rows, err := d.QueryContext(ctx, `SELECT purpose, COALESCE(SUM(cost_usd),0), COUNT(*) FROM claude_usage WHERE created_at >= ? GROUP BY purpose ORDER BY 2 DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*SpendByPurpose
	for rows.Next() {
		var s SpendByPurpose
		if err := rows.Scan(&s.Purpose, &s.CostUSD, &s.Requests); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}
