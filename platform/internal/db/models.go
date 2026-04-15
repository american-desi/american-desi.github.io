package db

import "time"

// Site represents a single niche website in the portfolio.
type Site struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	Domain           string    `json:"domain"`
	Niche            string    `json:"niche"`
	Tagline          string    `json:"tagline"`
	Description      string    `json:"description"`
	SeedKeywords     []string  `json:"seed_keywords"`
	AffiliateProgram []string  `json:"affiliate_program"`
	AdNetwork        string    `json:"ad_network"`
	AdsTxt           string    `json:"ads_txt"`
	Status           string    `json:"status"`
	HealthScore      float64   `json:"health_score"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Keyword is a single targeted search term for a site.
type Keyword struct {
	ID           int64     `json:"id"`
	SiteID       string    `json:"site_id"`
	Keyword      string    `json:"keyword"`
	Intent       string    `json:"intent"`
	Cluster      string    `json:"cluster"`
	SearchVolume int       `json:"search_volume"`
	Difficulty   float64   `json:"difficulty"`
	Covered      bool      `json:"covered"`
	Priority     int       `json:"priority"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

// AffiliateLink represents an embedded affiliate callout.
type AffiliateLink struct {
	Anchor         string `json:"anchor"`
	PlaceholderURL string `json:"placeholder_url"`
	Program        string `json:"program"`
	UTM            string `json:"utm"`
	Position       string `json:"position"` // header | in_content | comparison_table | footer
}

// Article is a single piece of content.
type Article struct {
	ID                 string          `json:"id"`
	SiteID             string          `json:"site_id"`
	Slug               string          `json:"slug"`
	Cluster            string          `json:"cluster"`
	Type               string          `json:"type"`
	Title              string          `json:"title"`
	MetaDescription    string          `json:"meta_description"`
	FocusKeyword       string          `json:"focus_keyword"`
	SecondaryKeywords  []string        `json:"secondary_keywords"`
	BodyHTML           string          `json:"body_html"`
	FAQJSONLD          string          `json:"faq_json_ld"`
	Author             string          `json:"author"`
	WordCount          int             `json:"word_count"`
	Status             string          `json:"status"`
	PublishedAt        *time.Time      `json:"published_at,omitempty"`
	LastOptimizedAt    *time.Time      `json:"last_optimized_at,omitempty"`
	GenerationCostUSD  float64         `json:"generation_cost_usd"`
	GenerationHash     string          `json:"generation_hash"`
	AffiliateLinks     []AffiliateLink `json:"affiliate_links"`
	InternalLinks      []string        `json:"internal_links"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// AnalyticsRow is a single day's Search Console pull.
type AnalyticsRow struct {
	ArticleID   string    `json:"article_id"`
	Date        time.Time `json:"date"`
	Impressions int       `json:"impressions"`
	Clicks      int       `json:"clicks"`
	CTR         float64   `json:"ctr"`
	AvgPosition float64   `json:"avg_position"`
	TopQuery    string    `json:"top_query"`
}

// Optimization logs one automated improvement.
type Optimization struct {
	ID             int64     `json:"id"`
	ArticleID      string    `json:"article_id"`
	Kind           string    `json:"kind"`
	Reason         string    `json:"reason"`
	BeforeSnapshot string    `json:"before_snapshot"`
	AfterSnapshot  string    `json:"after_snapshot"`
	BeforeMetrics  string    `json:"before_metrics"`
	AfterMetrics   string    `json:"after_metrics"`
	CostUSD        float64   `json:"cost_usd"`
	CreatedAt      time.Time `json:"created_at"`
}

// RevenueRow is daily revenue per site + source.
type RevenueRow struct {
	SiteID      string    `json:"site_id"`
	Date        time.Time `json:"date"`
	Source      string    `json:"source"`
	RevenueUSD  float64   `json:"revenue_usd"`
	Clicks      int       `json:"clicks"`
	Conversions int       `json:"conversions"`
	Sessions    int       `json:"sessions"`
	RawPayload  string    `json:"raw_payload"`
}

// PipelineItem is a queued article generation task.
type PipelineItem struct {
	ID            int64     `json:"id"`
	SiteID        string    `json:"site_id"`
	FocusKeyword  string    `json:"focus_keyword"`
	Cluster       string    `json:"cluster"`
	ArticleType   string    `json:"article_type"`
	ScheduledFor  time.Time `json:"scheduled_for"`
	Status        string    `json:"status"`
	ArticleID     string    `json:"article_id"`
	Attempts      int       `json:"attempts"`
	LastError     string    `json:"last_error"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NicheAnalysis is the structured scoring output.
type NicheAnalysis struct {
	Niche             string   `json:"niche"`
	MonthlySearchVol  int      `json:"monthly_search_vol"`
	CompetitionLevel  string   `json:"competition_level"`
	MonetizationPaths []string `json:"monetization_paths"`
	AvgAffiliateComm  float64  `json:"avg_affiliate_comm"`
	EstRPM            float64  `json:"est_rpm"`
	ContentVelocity   int      `json:"content_velocity"`
	TimeToRevenue     string   `json:"time_to_revenue"`
	Score             float64  `json:"score"`
	Rationale         string   `json:"rationale"`
}

// ClaudeUsage is an accounting row for API spend.
type ClaudeUsage struct {
	RequestID    string    `json:"request_id"`
	Purpose      string    `json:"purpose"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	CreatedAt    time.Time `json:"created_at"`
}
