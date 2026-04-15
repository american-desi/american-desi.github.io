# AI Content Revenue Platform

Autonomous AI-powered content publishing platform. Generates, publishes, and
optimizes SEO content across a portfolio of niche sites for passive revenue via
affiliate marketing and display advertising.

## Architecture

| Layer              | Technology                        |
|--------------------|-----------------------------------|
| Backend API        | Go 1.22 (net/http)                |
| Database           | SQLite (WAL mode, modernc.org)    |
| AI engine          | Anthropic Claude (sonnet-4)       |
| Reverse proxy      | Caddy (auto-HTTPS)                |
| Admin dashboard    | Solid.js + Vite                   |
| Static sites       | Hand-rolled HTML templates        |
| Hosting            | Cloudflare Pages / Netlify (free) |
| Infra              | Single $10 VPS (Ubuntu 22.04)     |

## Layout

```
platform/
├── cmd/server/          # HTTP entrypoint
├── internal/
│   ├── api/             # HTTP handlers + middleware
│   ├── claude/          # Anthropic API client
│   ├── config/          # Environment config loader
│   ├── db/              # SQLite + migrations
│   ├── generator/       # Article + site generators
│   ├── logging/         # Structured JSON logger
│   ├── niche/           # Niche profitability analyzer
│   ├── pipeline/        # Content scheduler
│   └── seo/             # Analytics + self-healing optimizer
├── dashboard/           # Solid.js admin UI
├── deploy/              # Caddyfile + systemd + bootstrap script
├── tests/               # Integration tests
└── data/                # SQLite DB + generated site output
```

## Quick start (dev)

```bash
cd platform
export ANTHROPIC_API_KEY=sk-ant-...
go run ./cmd/server
# API now on :8080, dashboard proxied from ./dashboard
```

## API Surface

| Method | Path                    | Purpose                              |
|--------|-------------------------|--------------------------------------|
| POST   | /api/generate           | Generate single article              |
| POST   | /api/site/create        | Generate complete niche site         |
| POST   | /api/site/publish       | Export static HTML to disk           |
| GET    | /api/sites              | List portfolio                       |
| GET    | /api/sites/:id          | Site detail                          |
| POST   | /api/niche/analyze      | Score a niche's profitability        |
| GET    | /api/analytics/summary  | SEO analytics summary                |
| POST   | /api/analytics/ingest   | Pull Search Console data             |
| POST   | /api/optimize/run       | Trigger self-healing cycle           |
| GET    | /api/optimize/log       | Optimization activity log            |
| POST   | /api/pipeline/enqueue   | Queue new article for publication    |
| GET    | /api/pipeline/status    | Pipeline state                       |
| GET    | /api/revenue/summary    | Revenue rollup per site              |
| GET    | /api/revenue/cost-report| Claude API spend vs revenue          |
| GET    | /healthz                | Liveness probe                       |
| GET    | /dashboard/*            | Solid.js admin UI                    |

See `STATUS.md` for coordination protocol with the Cowork agent.
