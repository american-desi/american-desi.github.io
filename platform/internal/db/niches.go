package db

import (
	"context"
	"database/sql"
	"encoding/json"
)

// UpsertNiche stores or updates a niche analysis.
func (d *DB) UpsertNiche(ctx context.Context, n *NicheAnalysis) error {
	paths, _ := json.Marshal(n.MonetizationPaths)
	_, err := d.ExecContext(ctx, `
		INSERT INTO niche_analyses (niche, monthly_search_vol, competition_level, monetization_paths, avg_affiliate_comm, est_rpm, content_velocity, time_to_revenue, score, rationale)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(niche) DO UPDATE SET
			monthly_search_vol = excluded.monthly_search_vol,
			competition_level = excluded.competition_level,
			monetization_paths = excluded.monetization_paths,
			avg_affiliate_comm = excluded.avg_affiliate_comm,
			est_rpm = excluded.est_rpm,
			content_velocity = excluded.content_velocity,
			time_to_revenue = excluded.time_to_revenue,
			score = excluded.score,
			rationale = excluded.rationale,
			analyzed_at = CURRENT_TIMESTAMP
	`, n.Niche, n.MonthlySearchVol, n.CompetitionLevel, string(paths), n.AvgAffiliateComm, n.EstRPM, n.ContentVelocity, n.TimeToRevenue, n.Score, n.Rationale)
	return err
}

// GetNiche loads a cached analysis or sql.ErrNoRows.
func (d *DB) GetNiche(ctx context.Context, niche string) (*NicheAnalysis, error) {
	row := d.QueryRowContext(ctx, `SELECT niche, monthly_search_vol, competition_level, COALESCE(monetization_paths,'[]'), avg_affiliate_comm, est_rpm, content_velocity, COALESCE(time_to_revenue,''), score, COALESCE(rationale,'') FROM niche_analyses WHERE niche = ?`, niche)
	return scanNiche(row)
}

// ListNiches returns all cached analyses sorted by score desc.
func (d *DB) ListNiches(ctx context.Context) ([]*NicheAnalysis, error) {
	rows, err := d.QueryContext(ctx, `SELECT niche, monthly_search_vol, competition_level, COALESCE(monetization_paths,'[]'), avg_affiliate_comm, est_rpm, content_velocity, COALESCE(time_to_revenue,''), score, COALESCE(rationale,'') FROM niche_analyses ORDER BY score DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*NicheAnalysis
	for rows.Next() {
		n, err := scanNiche(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func scanNiche(r rowScanner) (*NicheAnalysis, error) {
	var (
		n         NicheAnalysis
		pathsJSON string
	)
	err := r.Scan(&n.Niche, &n.MonthlySearchVol, &n.CompetitionLevel, &pathsJSON, &n.AvgAffiliateComm, &n.EstRPM, &n.ContentVelocity, &n.TimeToRevenue, &n.Score, &n.Rationale)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(pathsJSON), &n.MonetizationPaths)
	return &n, nil
}

// scanNicheRow lets callers share the scanner for *sql.Rows too.
var _ rowScanner = (*sql.Rows)(nil)
