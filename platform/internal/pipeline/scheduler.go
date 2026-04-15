// Package pipeline drives drip publishing: it picks up due queue items, calls
// the generator, marks articles as published, and replenishes the queue from
// uncovered keywords.
package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/generator"
	"github.com/american-desi/platform/internal/logging"
)

// Scheduler owns the drip publishing loop.
type Scheduler struct {
	db     *db.DB
	eng    *generator.Engine
	log    *logging.Logger
}

// New returns a Scheduler.
func New(database *db.DB, eng *generator.Engine, log *logging.Logger) *Scheduler {
	return &Scheduler{db: database, eng: eng, log: log}
}

// ProcessOnce runs one pass: generate due items, then replenish queue if it's low.
func (s *Scheduler) ProcessOnce(ctx context.Context) (processed int, err error) {
	due, err := s.db.DuePipelineItems(ctx, 5)
	if err != nil {
		return 0, err
	}
	for _, p := range due {
		// Mark generating so concurrent workers don't grab it.
		if err := s.db.UpdatePipelineStatus(ctx, p.ID, "generating", "", ""); err != nil {
			s.log.Warn(ctx, "pipeline: status update failed", map[string]any{"id": p.ID, "err": err.Error()})
			continue
		}
		art, genErr := s.eng.GenerateArticle(ctx, generator.GenerateArticleRequest{
			SiteID:       p.SiteID,
			FocusKeyword: p.FocusKeyword,
			Cluster:      p.Cluster,
			ArticleType:  p.ArticleType,
		})
		if genErr != nil {
			_ = s.db.UpdatePipelineStatus(ctx, p.ID, "failed", "", genErr.Error())
			s.log.Error(ctx, "pipeline: generate failed", map[string]any{"pipeline_id": p.ID, "err": genErr.Error()})
			continue
		}
		now := time.Now().UTC()
		// Publish: mark article status + publish time.
		_ = s.db.UpdateArticleStatus(ctx, art.ID, "published", &now)
		_ = s.db.UpdatePipelineStatus(ctx, p.ID, "published", art.ID, "")
		processed++
		s.log.Info(ctx, "pipeline: article published", map[string]any{
			"pipeline_id": p.ID,
			"article_id":  art.ID,
			"site_id":     art.SiteID,
			"focus":       art.FocusKeyword,
		})
	}

	// Replenish queues (2-3 items per site, drip-spaced 2-3 days apart).
	if err := s.replenishAll(ctx); err != nil {
		s.log.Warn(ctx, "pipeline: replenish failed", map[string]any{"err": err.Error()})
	}
	return processed, nil
}

// replenishAll ensures every live site has 3 queued items scheduled into the future.
func (s *Scheduler) replenishAll(ctx context.Context) error {
	sites, err := s.db.ListSites(ctx)
	if err != nil {
		return err
	}
	for _, site := range sites {
		if site.Status != "live" && site.Status != "draft" {
			continue
		}
		queued, err := s.pendingForSite(ctx, site.ID)
		if err != nil {
			continue
		}
		if queued >= 3 {
			continue
		}
		need := 3 - queued
		kws, err := s.db.UncoveredKeywords(ctx, site.ID, need*2)
		if err != nil || len(kws) == 0 {
			continue
		}
		// Drip schedule: next slot every 2-3 days from latest queued item.
		base := time.Now().UTC()
		for i := 0; i < need && i < len(kws); i++ {
			gap := time.Duration(48+24*i) * time.Hour
			sched := base.Add(gap)
			_, err := s.db.EnqueuePipelineItem(ctx, &db.PipelineItem{
				SiteID:       site.ID,
				FocusKeyword: kws[i].Keyword,
				Cluster:      kws[i].Cluster,
				ArticleType:  inferArticleType(kws[i].Intent),
				ScheduledFor: sched,
				Status:       "queued",
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func inferArticleType(intent string) string {
	switch intent {
	case "commercial":
		return "review"
	case "transactional":
		return "comparison"
	default:
		return "article"
	}
}

// pendingForSite counts queued or generating items for a site.
func (s *Scheduler) pendingForSite(ctx context.Context, siteID string) (int, error) {
	var n int
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipeline WHERE site_id = ? AND status IN ('queued','generating')`, siteID)
	err := row.Scan(&n)
	return n, err
}

// Loop runs ProcessOnce on a ticker until ctx cancels.
func (s *Scheduler) Loop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			func() {
				runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
				defer cancel()
				runCtx = logging.WithRequestID(runCtx, fmt.Sprintf("pipeline-%s", time.Now().UTC().Format("20060102-150405")))
				if _, err := s.ProcessOnce(runCtx); err != nil {
					s.log.Error(runCtx, "pipeline: tick failed", map[string]any{"err": err.Error()})
				}
			}()
		}
	}
}
