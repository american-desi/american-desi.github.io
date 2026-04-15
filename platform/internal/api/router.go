package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/american-desi/platform/internal/claude"
	"github.com/american-desi/platform/internal/config"
	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/generator"
	"github.com/american-desi/platform/internal/logging"
	"github.com/american-desi/platform/internal/niche"
	"github.com/american-desi/platform/internal/pipeline"
	"github.com/american-desi/platform/internal/seo"
)

// Server bundles all dependencies the handlers need.
type Server struct {
	Config    *config.Config
	Log       *logging.Logger
	DB        *db.DB
	Claude    *claude.Client
	Generator *generator.Engine
	Analytics *seo.Analytics
	Optimizer *seo.Optimizer
	Pipeline  *pipeline.Scheduler
	Niche     *niche.Analyzer
}

// Handler returns the root http.Handler with middleware attached.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.register(mux)

	rl := newRateLimitMW(s.Config.PublicRPS)

	// Middleware chain: recover → request-id → logging → rate limit.
	var h http.Handler = mux
	h = rl.handler(h)
	h = loggingMW(s.Log)(h)
	h = requestIDMW(h)
	h = recoverMW(s.Log)(h)
	return h
}

// register wires routes. We use net/http's ServeMux with Go 1.22 pattern syntax.
func (s *Server) register(mux *http.ServeMux) {
	// Health.
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)

	// Content generation.
	mux.HandleFunc("POST /api/generate", s.generateArticle)

	// Sites.
	mux.HandleFunc("POST /api/site/create", s.createSite)
	mux.HandleFunc("POST /api/site/publish", s.publishSite)
	mux.HandleFunc("GET /api/sites", s.listSites)
	mux.HandleFunc("GET /api/sites/{id}", s.getSite)

	// Analytics.
	mux.HandleFunc("GET /api/analytics/summary", s.analyticsSummary)
	mux.HandleFunc("POST /api/analytics/ingest", s.analyticsIngest)
	mux.HandleFunc("POST /api/analytics/pull", s.analyticsPull)

	// Optimizer.
	mux.HandleFunc("POST /api/optimize/run", s.optimizeRun)
	mux.HandleFunc("GET /api/optimize/log", s.optimizeLog)

	// Pipeline.
	mux.HandleFunc("POST /api/pipeline/enqueue", s.pipelineEnqueue)
	mux.HandleFunc("POST /api/pipeline/tick", s.pipelineTick)
	mux.HandleFunc("GET /api/pipeline/status", s.pipelineStatus)

	// Niche.
	mux.HandleFunc("POST /api/niche/analyze", s.nicheAnalyze)
	mux.HandleFunc("GET /api/niche/list", s.nicheList)

	// Revenue.
	mux.HandleFunc("GET /api/revenue/summary", s.revenueSummary)
	mux.HandleFunc("POST /api/revenue/ingest", s.revenueIngest)
	mux.HandleFunc("GET /api/revenue/cost-report", s.revenueCostReport)

	// Articles (for dashboard inspection).
	mux.HandleFunc("GET /api/articles/{site_id}", s.listArticles)
	mux.HandleFunc("GET /api/article/{id}", s.getArticle)

	// Dashboard static files + SPA fallback.
	mux.HandleFunc("/", s.dashboardHandler)
}

// dashboardHandler serves the Solid.js admin panel static files with SPA
// fallback to index.html. If the dashboard hasn't been built, it serves a
// bootstrap page with a link to the API.
func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	// If someone hits an unknown /api path, 404.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	dir := s.Config.DashboardDir
	if dir == "" {
		dir = "./dashboard/dist"
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Fall back to built-in bootstrap page.
		w.Header().Set("content-type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(bootstrapHTML))
		return
	}
	// Clean path, prevent traversal.
	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	reqPath = filepath.Clean(reqPath)
	if strings.HasPrefix(reqPath, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(dir, reqPath)
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	// SPA fallback.
	http.ServeFile(w, r, filepath.Join(dir, "index.html"))
}

const bootstrapHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>AI Content Platform — Admin</title>
<style>body{font-family:system-ui,sans-serif;max-width:720px;margin:3rem auto;padding:0 1rem;color:#111;line-height:1.6}code{background:#f5f5f5;padding:.15rem .4rem;border-radius:4px}.warn{background:#fff7ed;padding:1rem;border-left:4px solid #f59e0b;border-radius:4px}</style>
</head><body>
<h1>AI Content Platform — Admin</h1>
<div class="warn">The Solid.js dashboard hasn't been built yet. Run <code>cd platform/dashboard && npm install && npm run build</code> to produce <code>dashboard/dist</code>.</div>
<h2>Core endpoints</h2>
<ul>
<li><code>GET /healthz</code> — liveness</li>
<li><code>GET /api/sites</code> — portfolio</li>
<li><code>POST /api/site/create</code> — create a niche site</li>
<li><code>POST /api/generate</code> — generate an article</li>
<li><code>GET /api/analytics/summary</code> — SEO health</li>
<li><code>POST /api/optimize/run</code> — trigger self-healing cycle</li>
<li><code>GET /api/revenue/summary</code> — revenue rollup</li>
<li><code>GET /api/revenue/cost-report</code> — Claude spend</li>
</ul>
<p>See <code>platform/README.md</code> for the full API surface.</p>
</body></html>`
