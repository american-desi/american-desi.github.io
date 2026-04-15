// Command server is the single entrypoint for the AI content platform.
// It boots the HTTP API, the pipeline scheduler, and the optimizer loop.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/american-desi/platform/internal/api"
	"github.com/american-desi/platform/internal/claude"
	"github.com/american-desi/platform/internal/config"
	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/generator"
	"github.com/american-desi/platform/internal/logging"
	"github.com/american-desi/platform/internal/niche"
	"github.com/american-desi/platform/internal/pipeline"
	"github.com/american-desi/platform/internal/seo"
)

func main() {
	log := logging.New()
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		log.SetLevel(logging.Level(v))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Error(ctx, "config load failed", map[string]any{"err": err.Error()})
		os.Exit(2)
	}

	if cfg.AnthropicAPIKey == "" {
		log.Warn(ctx, "ANTHROPIC_API_KEY not set — content generation endpoints will error until configured", nil)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error(ctx, "db open failed", map[string]any{"err": err.Error(), "path": cfg.DBPath})
		os.Exit(2)
	}
	defer database.Close()

	cc := claude.New(claude.Config{
		APIKey:      cfg.AnthropicAPIKey,
		Model:       cfg.ClaudeModel,
		MaxTokens:   cfg.ClaudePerRequestMaxTokens,
		Timeout:     cfg.ClaudeRequestTimeout,
		BudgetUSD:   cfg.ClaudeMonthlyBudgetUSD,
		Concurrency: cfg.ClaudeConcurrency,
	}, database, log)

	gscClient := seo.NewGSCClient(cfg.GSCCredentialsJSON)

	gen := generator.NewEngine(cc, database, log)
	analytics := seo.NewAnalytics(database, log, gscClient)
	optimizer := seo.NewOptimizer(database, cc, log, cfg.MinArticleAgeDays)
	nicher := niche.New(database, cc, log)
	sched := pipeline.New(database, gen, log)

	srv := &api.Server{
		Config:    cfg,
		Log:       log,
		DB:        database,
		Claude:    cc,
		Generator: gen,
		Analytics: analytics,
		Optimizer: optimizer,
		Pipeline:  sched,
		Niche:     nicher,
	}

	// Start background loops.
	go sched.Loop(ctx, cfg.PipelineTickInterval)
	go optimizer.Loop(ctx, cfg.OptimizerTickInterval)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Info(ctx, "http: listening", map[string]any{
			"addr":              cfg.HTTPAddr,
			"env":               cfg.Env,
			"claude_model":      cfg.ClaudeModel,
			"budget_cap_usd":    cfg.ClaudeMonthlyBudgetUSD,
			"pipeline_tick":     cfg.PipelineTickInterval.String(),
			"optimizer_tick":    cfg.OptimizerTickInterval.String(),
			"gsc_configured":    gscClient.Configured(),
		})
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error(ctx, "http: fatal", map[string]any{"err": err.Error()})
			stop()
		}
	}()

	<-ctx.Done()
	log.Info(ctx, "shutdown: signal received", nil)

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Warn(shutCtx, "http: shutdown error", map[string]any{"err": err.Error()})
	}
	fmt.Fprintln(os.Stderr, "bye")
}
