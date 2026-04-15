# PROJECT STATUS

## State: READY_FOR_DEPLOY

## Current Phase: Platform build complete. Awaiting operator credentials (Anthropic API key, domain, GSC) to go live.

## Build Verification (as of this commit):
- [x] `go build ./...` clean
- [x] `go vet ./...` clean
- [x] `go test ./...` all passing (db, claude, generator, api packages)
- [x] Server boots + serves /healthz, /readyz, and 20+ API routes
- [x] 10 starter niches pre-seeded in DB via migration 0002
- [x] DB migrations embedded via `go:embed` (single-binary deploy)
- [x] Site creation endpoint working end-to-end (verified in smoke test)
- [x] Structured JSON logging to stdout with request IDs
- [x] Rate limiting, panic recovery, graceful shutdown all wired

## Operator Requests:

### Critical (blocks revenue)
- [ ] NEED_API_KEY: Anthropic Claude API — content generation + optimization + niche analysis — env var `ANTHROPIC_API_KEY`
- [ ] NEED_API_KEY: Google Search Console — SEO analytics ingestion — env var `GSC_CREDENTIALS_JSON` (path to OAuth service account JSON)
- [ ] NEED_DOMAIN: personal-finance-tools niche — suggested: `finedgepro.com`, `walletforge.io`, `ledgerloop.com` — Porkbun or Cloudflare Registrar (cheapest .com + free WHOIS)
- [ ] NEED_DOMAIN: home-office-gear niche — suggested: `deskcraft.io`, `workspaceverdict.com`, `homerigged.com` — Porkbun or Cloudflare Registrar
- [ ] NEED_DEPLOY_SITE: first niche site — files in `platform/data/sites/<slug>/public/` — point domain to Cloudflare Pages or Netlify (free tier)

### Affiliate program signups (one per niche site, in order of niche priority)
- [ ] NEED_AFFILIATE: Amazon Associates — https://affiliate-program.amazon.com — default for all niche sites (broad product coverage)
- [ ] NEED_AFFILIATE: Impact.com — https://app.impact.com/campaign-promo-signup — personal finance, VPN, hosting niches
- [ ] NEED_AFFILIATE: ShareASale — https://shareasale.com/newsignup.cfm — home office, baby gear, fitness niches
- [ ] NEED_AFFILIATE: CJ Affiliate — https://signup.cj.com — multi-niche fallback
- [ ] NEED_AFFILIATE: Awin — https://www.awin.com/us/publishers/sign-up — multi-niche fallback

### Ad network signups (trigger when site traffic meets thresholds)
- [ ] NEED_AD_NETWORK: Ezoic — any URL, 0 traffic threshold — start here, no minimums
- [ ] NEED_AD_NETWORK: Mediavine — 50k sessions/month threshold — upgrade path
- [ ] NEED_AD_NETWORK: AdThrive — 100k pageviews/month threshold — premium tier

### Infrastructure
- [ ] NEED_SIGNUP: Cloudflare (Pages + DNS + Registrar) — free tier — account email + API token for DNS automation
- [ ] NEED_SIGNUP: Hetzner or DigitalOcean — $10 VPS Ubuntu 22.04 — SSH key already configured, need IP + root credentials
- [ ] NEED_SIGNUP: Google Search Console — per-domain verification — TXT record needed once domains live

## Operator Completed:
<!-- Cowork writes results here. Expected format:
- [x] NEED_API_KEY: Anthropic — key added to .env on VPS — verified working on YYYY-MM-DD
-->

## Revenue Report:
<!-- Cowork writes weekly revenue data here. Expected format:
Week of YYYY-MM-DD:
- site1.com: $X.XX affiliate, $X.XX ads, X sessions
- site2.com: $X.XX affiliate, $X.XX ads, X sessions
Total: $X.XX
-->

## Content Performance:
<!-- Auto-populated by SEO analytics engine after Search Console connected -->

## Deploy Checklist:
- [ ] VPS provisioned ($10 tier)
- [ ] Caddy installed + HTTPS working
- [ ] Admin dashboard deployed on VPS
- [ ] `ANTHROPIC_API_KEY` configured
- [ ] Google Search Console API connected
- [ ] SQLite DB initialized with migrations
- [ ] Weekly optimization cron running
- [ ] First niche site generated and deployed
- [ ] At least one affiliate account active
- [ ] Ezoic account approved + ads.txt deployed
- [ ] Content pipeline publishing 2-3 articles/week per site

## Local Dev Quick-Start (for operator smoke-testing before VPS):
```bash
cd platform
go build -o /tmp/platform-server ./cmd/server
ANTHROPIC_API_KEY=sk-ant-... DATA_DIR=./data /tmp/platform-server
# then: curl http://localhost:8080/api/niche/list
```

To build the dashboard:
```bash
cd platform/dashboard && npm install && npm run build
# Dashboard served from dashboard/dist by the Go server automatically.
```

## Handoff Notes:
- Claude API is the single largest variable cost. Monitor spend via `/api/revenue/cost-report`. Budget cap defaults to $50/month; raise via `CLAUDE_MONTHLY_BUDGET_USD` env var.
- All affiliate links are generated with UTM params (`utm_source=<site>&utm_medium=affiliate&utm_campaign=<cluster>`) for per-article ROI attribution.
- Starter niche order (highest expected RPM first): personal-finance-tools → home-office-gear → vpn-privacy → web-hosting → online-learning. Revisit after 8 weeks of data.
- Sites are intentionally independent: no interlinking, separate WHOIS, separate IP blocks via Cloudflare Pages. This is a deliberate privateblog-network-prevention choice.
- The self-healing optimizer will NOT touch articles younger than 60 days (Google sandbox window). Override with `?force=true` on `/api/optimize/run`.
