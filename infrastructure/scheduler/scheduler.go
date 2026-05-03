package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	appCrawler "github.com/atvirokodosprendimai/crawlerv2/application/crawler"
	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron      *cron.Cron
	triggerUC *appCrawler.TriggerJobUseCase
	reclaimUC *appCrawler.ReclaimStaleTasksUseCase
	staleTO   time.Duration

	mu      sync.Mutex
	entries map[uint]cron.EntryID // domainID → cron entry
}

func New(
	trigger *appCrawler.TriggerJobUseCase,
	reclaim *appCrawler.ReclaimStaleTasksUseCase,
	staleTimeout time.Duration,
) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		triggerUC: trigger,
		reclaimUC: reclaim,
		staleTO:   staleTimeout,
		entries:   make(map[uint]cron.EntryID),
	}
}

// Register adds or replaces a cron job for a domain.
func (s *Scheduler) Register(d *domain.CrawlDomain) {
	if d.CronExpression == "" {
		s.Unregister(d.ID)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.entries[d.ID]; ok {
		s.cron.Remove(old)
	}

	domainID := d.ID
	entryID, err := s.cron.AddFunc(d.CronExpression, func() {
		ctx := context.Background()
		_, err := s.triggerUC.Execute(ctx, appCrawler.TriggerJobInput{DomainID: domainID})
		if err != nil {
			log.Printf("scheduler: domain %d: %v", domainID, err)
		}
	})
	if err != nil {
		log.Printf("scheduler: invalid cron for domain %d: %v", domainID, err)
		return
	}
	s.entries[domainID] = entryID
	fmt.Printf("scheduler: domain %d registered cron %q\n", d.ID, d.CronExpression)
}

// Unregister removes a domain's cron job.
func (s *Scheduler) Unregister(domainID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.entries[domainID]; ok {
		s.cron.Remove(id)
		delete(s.entries, domainID)
	}
}

// Start starts the cron runner and the stale-reclaim ticker.
func (s *Scheduler) Start(ctx context.Context) {
	s.cron.Start()

	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				n, err := s.reclaimUC.Execute(ctx, s.staleTO)
				if err != nil {
					log.Printf("reclaim stale: %v", err)
				} else if n > 0 {
					log.Printf("reclaimed %d stale tasks", n)
				}
			}
		}
	}()
}

// Stop stops the cron runner gracefully.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}
