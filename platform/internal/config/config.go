// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration loaded from env vars.
type Config struct {
	// Server
	HTTPAddr     string
	DataDir      string
	SitesDir     string
	DBPath       string
	DashboardDir string

	// Claude
	AnthropicAPIKey       string
	ClaudeModel           string
	ClaudeRequestTimeout  time.Duration
	ClaudeMonthlyBudgetUSD float64
	ClaudePerRequestMaxTokens int

	// Rate limits
	PublicRPS         float64
	ClaudeConcurrency int

	// Search Console
	GSCCredentialsJSON string

	// Pipeline
	PipelineTickInterval time.Duration
	OptimizerTickInterval time.Duration
	MinArticleAgeDays     int // articles younger than this won't be auto-optimized

	// Affiliate defaults
	DefaultUTMSource string
	DefaultUTMMedium string

	// Environment flag
	Env string
}

// Load returns config populated from the process environment with sane defaults.
func Load() (*Config, error) {
	c := &Config{
		HTTPAddr:                  getEnv("HTTP_ADDR", ":8080"),
		DataDir:                   getEnv("DATA_DIR", "./data"),
		ClaudeModel:               getEnv("CLAUDE_MODEL", "claude-sonnet-4-20250514"),
		ClaudeRequestTimeout:      getDuration("CLAUDE_TIMEOUT", 90*time.Second),
		ClaudeMonthlyBudgetUSD:    getFloat("CLAUDE_MONTHLY_BUDGET_USD", 50.0),
		ClaudePerRequestMaxTokens: getInt("CLAUDE_MAX_TOKENS", 8192),
		PublicRPS:                 getFloat("PUBLIC_RPS", 5),
		ClaudeConcurrency:         getInt("CLAUDE_CONCURRENCY", 3),
		AnthropicAPIKey:           os.Getenv("ANTHROPIC_API_KEY"),
		GSCCredentialsJSON:        os.Getenv("GSC_CREDENTIALS_JSON"),
		PipelineTickInterval:      getDuration("PIPELINE_TICK", 30*time.Minute),
		OptimizerTickInterval:     getDuration("OPTIMIZER_TICK", 7*24*time.Hour),
		MinArticleAgeDays:         getInt("MIN_ARTICLE_AGE_DAYS", 60),
		DefaultUTMSource:          getEnv("UTM_SOURCE", "{site}"),
		DefaultUTMMedium:          getEnv("UTM_MEDIUM", "affiliate"),
		Env:                       getEnv("ENV", "development"),
	}
	c.DBPath = getEnv("DB_PATH", c.DataDir+"/platform.db")
	c.SitesDir = getEnv("SITES_DIR", c.DataDir+"/sites")
	c.DashboardDir = getEnv("DASHBOARD_DIR", "./dashboard/dist")

	// Soft validation: API key may be empty in dev but we log a warning elsewhere.
	if strings.TrimSpace(c.ClaudeModel) == "" {
		return nil, fmt.Errorf("CLAUDE_MODEL is empty")
	}
	return c, nil
}

func getEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getFloat(k string, def float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func getDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
