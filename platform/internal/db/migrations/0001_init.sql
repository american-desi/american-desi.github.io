-- Migration 0001: Initial schema for AI content revenue platform.
-- Executed once on first startup.
-- (PRAGMAs are applied on the connection via DSN in db.Open; they cannot
--  run inside a transaction here.)

-- Tracks schema version.
CREATE TABLE IF NOT EXISTS schema_migrations (
    version     INTEGER PRIMARY KEY,
    applied_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Portfolio of niche sites.
CREATE TABLE IF NOT EXISTS sites (
    id                TEXT PRIMARY KEY,              -- uuid
    slug              TEXT UNIQUE NOT NULL,          -- e.g. "personal-finance-tools"
    domain            TEXT,                          -- e.g. "walletforge.io"  (nullable until operator registers)
    niche             TEXT NOT NULL,
    tagline           TEXT,
    description       TEXT,
    seed_keywords     TEXT,                          -- JSON array
    affiliate_program TEXT,                          -- JSON array of program codes
    ad_network        TEXT,                          -- e.g. "ezoic", "mediavine"
    ads_txt           TEXT,                          -- raw ads.txt content
    status            TEXT NOT NULL DEFAULT 'draft', -- draft | live | paused
    health_score      REAL NOT NULL DEFAULT 1.0,     -- 0.0-1.0
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sites_slug ON sites(slug);
CREATE INDEX IF NOT EXISTS idx_sites_status ON sites(status);

-- Keyword map per site (drives content planning).
CREATE TABLE IF NOT EXISTS keywords (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id          TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    keyword          TEXT NOT NULL,
    intent           TEXT NOT NULL,        -- informational | commercial | transactional
    cluster          TEXT,                 -- groups related keywords
    search_volume    INTEGER DEFAULT 0,
    difficulty       REAL DEFAULT 0,       -- 0-100
    covered          INTEGER DEFAULT 0,    -- 1 if an article targets it
    priority         INTEGER DEFAULT 5,    -- 1 (high) - 10 (low)
    discovered_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, keyword)
);

CREATE INDEX IF NOT EXISTS idx_keywords_site ON keywords(site_id);
CREATE INDEX IF NOT EXISTS idx_keywords_covered ON keywords(site_id, covered, priority);

-- Articles: the core revenue unit.
CREATE TABLE IF NOT EXISTS articles (
    id                 TEXT PRIMARY KEY,              -- uuid
    site_id            TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    slug               TEXT NOT NULL,                 -- url path segment
    cluster            TEXT,                          -- pillar | supporting | review | comparison
    type               TEXT NOT NULL DEFAULT 'article',
    title              TEXT NOT NULL,                 -- <= 60 chars for SEO
    meta_description   TEXT NOT NULL,                 -- <= 155 chars
    focus_keyword      TEXT NOT NULL,
    secondary_keywords TEXT,                          -- JSON array
    body_html          TEXT NOT NULL,                 -- semantic HTML
    faq_json_ld        TEXT,                          -- JSON-LD FAQ schema
    author             TEXT NOT NULL DEFAULT 'Editorial Team',
    word_count         INTEGER NOT NULL DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'draft', -- draft | scheduled | published | optimizing
    published_at       TIMESTAMP,
    last_optimized_at  TIMESTAMP,
    generation_cost_usd REAL DEFAULT 0,
    generation_hash     TEXT,                         -- hash of seed inputs for idempotency
    affiliate_links_json TEXT,                        -- JSON array of {anchor, placeholder_url, program, utm}
    internal_links_json  TEXT,                        -- JSON array of article_ids
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(site_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_articles_site ON articles(site_id);
CREATE INDEX IF NOT EXISTS idx_articles_status ON articles(status);
CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at);
CREATE INDEX IF NOT EXISTS idx_articles_hash ON articles(generation_hash);

-- Per-article, per-day analytics pulled from Search Console.
CREATE TABLE IF NOT EXISTS analytics (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id     TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    date           DATE NOT NULL,
    impressions    INTEGER NOT NULL DEFAULT 0,
    clicks         INTEGER NOT NULL DEFAULT 0,
    ctr            REAL NOT NULL DEFAULT 0,
    avg_position   REAL NOT NULL DEFAULT 0,
    top_query      TEXT,
    UNIQUE(article_id, date)
);

CREATE INDEX IF NOT EXISTS idx_analytics_article_date ON analytics(article_id, date);

-- Self-healing optimization log: records every automated change.
CREATE TABLE IF NOT EXISTS optimizations (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id      TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,             -- rewrite | meta_refresh | content_refresh | cannibalization_merge
    reason          TEXT NOT NULL,             -- striking_distance | low_ctr | declining | cannibalization
    before_snapshot TEXT,                      -- JSON snapshot
    after_snapshot  TEXT,
    before_metrics  TEXT,                      -- JSON {impressions, clicks, ctr, position}
    after_metrics   TEXT,                      -- filled in after 14d
    cost_usd        REAL DEFAULT 0,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    measured_at     TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_opt_article ON optimizations(article_id);
CREATE INDEX IF NOT EXISTS idx_opt_created ON optimizations(created_at);

-- Revenue per site per day (manual ingestion + future automated pulls).
CREATE TABLE IF NOT EXISTS revenue (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id         TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    date            DATE NOT NULL,
    source          TEXT NOT NULL,             -- amazon | impact | shareasale | ezoic | mediavine | ...
    revenue_usd     REAL NOT NULL DEFAULT 0,
    clicks          INTEGER DEFAULT 0,
    conversions     INTEGER DEFAULT 0,
    sessions        INTEGER DEFAULT 0,
    raw_payload     TEXT,                      -- JSON from Cowork
    UNIQUE(site_id, date, source)
);

CREATE INDEX IF NOT EXISTS idx_revenue_site_date ON revenue(site_id, date);

-- Content pipeline queue (drip publishing).
CREATE TABLE IF NOT EXISTS pipeline (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    site_id       TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    focus_keyword TEXT NOT NULL,
    cluster       TEXT,
    article_type  TEXT NOT NULL DEFAULT 'article', -- article | review | comparison | pillar
    scheduled_for TIMESTAMP NOT NULL,
    status        TEXT NOT NULL DEFAULT 'queued', -- queued | generating | published | failed
    article_id    TEXT REFERENCES articles(id) ON DELETE SET NULL,
    attempts      INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pipeline_site_sched ON pipeline(site_id, scheduled_for);
CREATE INDEX IF NOT EXISTS idx_pipeline_status ON pipeline(status, scheduled_for);

-- Niche profitability analysis cache.
CREATE TABLE IF NOT EXISTS niche_analyses (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    niche               TEXT UNIQUE NOT NULL,
    monthly_search_vol  INTEGER NOT NULL DEFAULT 0,
    competition_level   TEXT NOT NULL DEFAULT 'medium',
    monetization_paths  TEXT,                    -- JSON array
    avg_affiliate_comm  REAL DEFAULT 0,
    est_rpm             REAL DEFAULT 0,
    content_velocity    INTEGER DEFAULT 0,
    time_to_revenue     TEXT,
    score               REAL DEFAULT 0,
    rationale           TEXT,
    analyzed_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Claude API cost ledger (for budget enforcement).
CREATE TABLE IF NOT EXISTS claude_usage (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id    TEXT NOT NULL,
    purpose       TEXT NOT NULL,        -- article_gen | optimize | niche | meta_rewrite | keyword_map
    model         TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd      REAL NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_claude_usage_created ON claude_usage(created_at);
CREATE INDEX IF NOT EXISTS idx_claude_usage_purpose ON claude_usage(purpose);

INSERT OR IGNORE INTO schema_migrations (version) VALUES (1);
