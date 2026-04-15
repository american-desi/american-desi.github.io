package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/generator"
	"github.com/american-desi/platform/internal/niche"
	"github.com/american-desi/platform/internal/seo"
)

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.PingContext(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "db unreachable")
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":         "ready",
		"claude_configured": s.Config.AnthropicAPIKey != "",
	})
}

// POST /api/generate
func (s *Server) generateArticle(w http.ResponseWriter, r *http.Request) {
	var req generator.GenerateArticleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	art, err := s.Generator.GenerateArticle(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, art)
}

// POST /api/site/create
func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var req generator.CreateSiteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	site, err := s.Generator.CreateSite(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 201, site)
}

// POST /api/site/publish
func (s *Server) publishSite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID string `json:"site_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SiteID == "" {
		writeError(w, http.StatusBadRequest, "site_id required")
		return
	}
	outDir, count, err := s.Generator.PublishSite(r.Context(), req.SiteID, s.Config.SitesDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"output_dir":     outDir,
		"article_count":  count,
	})
}

// GET /api/sites
func (s *Server) listSites(w http.ResponseWriter, r *http.Request) {
	sites, err := s.DB.ListSites(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, sites)
}

// GET /api/sites/{id}
func (s *Server) getSite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	site, err := s.DB.GetSiteByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "site not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, site)
}

// GET /api/analytics/summary
func (s *Server) analyticsSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.Analytics.BuildSummary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, summary)
}

// POST /api/analytics/ingest
func (s *Server) analyticsIngest(w http.ResponseWriter, r *http.Request) {
	var p seo.IngestPayload
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	n, err := s.Analytics.Ingest(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"rows": n})
}

// POST /api/analytics/pull
func (s *Server) analyticsPull(w http.ResponseWriter, r *http.Request) {
	days := 28
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	n, err := s.Analytics.PullFromGSC(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"rows_ingested": n, "days": days})
}

// POST /api/optimize/run
func (s *Server) optimizeRun(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true"
	res, err := s.Optimizer.Run(r.Context(), force)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, res)
}

// GET /api/optimize/log
func (s *Server) optimizeLog(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	rows, err := s.DB.ListOptimizations(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

// POST /api/pipeline/enqueue
func (s *Server) pipelineEnqueue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteID       string    `json:"site_id"`
		FocusKeyword string    `json:"focus_keyword"`
		Cluster      string    `json:"cluster"`
		ArticleType  string    `json:"article_type"`
		ScheduledFor time.Time `json:"scheduled_for"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SiteID == "" || req.FocusKeyword == "" {
		writeError(w, http.StatusBadRequest, "site_id and focus_keyword required")
		return
	}
	if req.ScheduledFor.IsZero() {
		req.ScheduledFor = time.Now().UTC()
	}
	id, err := s.DB.EnqueuePipelineItem(r.Context(), &db.PipelineItem{
		SiteID:       req.SiteID,
		FocusKeyword: req.FocusKeyword,
		Cluster:      req.Cluster,
		ArticleType:  req.ArticleType,
		ScheduledFor: req.ScheduledFor,
		Status:       "queued",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

// POST /api/pipeline/tick — manual trigger for one pipeline cycle.
func (s *Server) pipelineTick(w http.ResponseWriter, r *http.Request) {
	n, err := s.Pipeline.ProcessOnce(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"processed": n})
}

// GET /api/pipeline/status
func (s *Server) pipelineStatus(w http.ResponseWriter, r *http.Request) {
	items, err := s.DB.ListPipelineItems(r.Context(), 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, items)
}

// POST /api/niche/analyze
func (s *Server) nicheAnalyze(w http.ResponseWriter, r *http.Request) {
	var req niche.Request
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.Niche.Analyze(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, res)
}

// GET /api/niche/list
func (s *Server) nicheList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListNiches(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

// GET /api/revenue/summary
func (s *Server) revenueSummary(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	rows, err := s.DB.RevenueSummary(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"days":  days,
		"sites": rows,
	})
}

// POST /api/revenue/ingest — Cowork pushes weekly rollups here.
func (s *Server) revenueIngest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rows []struct {
			SiteID      string  `json:"site_id"`
			SiteSlug    string  `json:"site_slug"`
			Date        string  `json:"date"`
			Source      string  `json:"source"`
			RevenueUSD  float64 `json:"revenue_usd"`
			Clicks      int     `json:"clicks"`
			Conversions int     `json:"conversions"`
			Sessions    int     `json:"sessions"`
			RawPayload  string  `json:"raw_payload"`
		} `json:"rows"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	n := 0
	for _, row := range req.Rows {
		siteID := row.SiteID
		if siteID == "" && row.SiteSlug != "" {
			site, err := s.DB.GetSiteBySlug(r.Context(), row.SiteSlug)
			if err != nil || site == nil {
				continue
			}
			siteID = site.ID
		}
		if siteID == "" || row.Source == "" || row.Date == "" {
			continue
		}
		d, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			continue
		}
		if err := s.DB.UpsertRevenue(r.Context(), &db.RevenueRow{
			SiteID:      siteID,
			Date:        d,
			Source:      row.Source,
			RevenueUSD:  row.RevenueUSD,
			Clicks:      row.Clicks,
			Conversions: row.Conversions,
			Sessions:    row.Sessions,
			RawPayload:  row.RawPayload,
		}); err == nil {
			n++
		}
	}
	writeJSON(w, 200, map[string]int{"rows": n})
}

// GET /api/revenue/cost-report — Claude API spend vs revenue.
func (s *Server) revenueCostReport(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	spend, err := s.DB.ClaudeSpendByPurpose(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	mtd, err := s.DB.MonthToDateSpend(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	revenue, err := s.DB.RevenueSummary(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	totalRev := 0.0
	for _, r := range revenue {
		totalRev += r.RevenueUSD
	}
	totalCost := 0.0
	for _, s := range spend {
		totalCost += s.CostUSD
	}
	writeJSON(w, 200, map[string]any{
		"days":            days,
		"spend_by_purpose": spend,
		"month_to_date_usd": mtd,
		"budget_cap_usd":    s.Config.ClaudeMonthlyBudgetUSD,
		"revenue_usd":       totalRev,
		"cost_usd":          totalCost,
		"roi":               safeDiv(totalRev, totalCost),
	})
}

// GET /api/articles/{site_id}
func (s *Server) listArticles(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("site_id")
	articles, err := s.DB.ListArticlesBySite(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, 200, articles)
}

// GET /api/article/{id}
func (s *Server) getArticle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := s.DB.GetArticle(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "article not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Option: pass ?include=body=false for dashboard listings.
	if strings.EqualFold(r.URL.Query().Get("include_body"), "false") {
		a.BodyHTML = ""
	}
	writeJSON(w, 200, a)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
