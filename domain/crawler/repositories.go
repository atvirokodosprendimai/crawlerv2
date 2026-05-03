package crawler

import (
	"context"
	"time"
)

type DomainRepository interface {
	Save(ctx context.Context, domain *CrawlDomain) error
	FindByID(ctx context.Context, id uint) (*CrawlDomain, error)
	FindAll(ctx context.Context) ([]CrawlDomain, error)
	Delete(ctx context.Context, id uint) error
}

type CrawlJobRepository interface {
	Create(ctx context.Context, job *CrawlJob) error
	UpdateStatus(ctx context.Context, jobID uint, status JobStatus) error
	IncrementPages(ctx context.Context, jobID uint, count int) error
	FindByID(ctx context.Context, id uint) (*CrawlJob, error)
	FindActive(ctx context.Context, domainID uint) (*CrawlJob, error)
	FindAll(ctx context.Context, domainID uint, status JobStatus) ([]CrawlJob, error)
	SetEnded(ctx context.Context, jobID uint) error
}

type URLRepository interface {
	Enqueue(ctx context.Context, urls []URLRecord) error
	ClaimBatch(ctx context.Context, jobID uint, batchSize int) ([]URLRecord, error)
	MarkDone(ctx context.Context, id uint) error
	MarkFailed(ctx context.Context, id uint, reason string) error
	ReclaimStale(ctx context.Context, threshold time.Duration) (int64, error)
	CountInProgress(ctx context.Context, rootDomain string) (int64, error)
	CountDone(ctx context.Context, jobID uint) (int64, error)
}

type CrawlResultRepository interface {
	Save(ctx context.Context, result *CrawlResult) error
	FindByJob(ctx context.Context, jobID uint) ([]CrawlResult, error)
}

type TokenRepository interface {
	Save(ctx context.Context, token *Token) error
	FindByHash(ctx context.Context, hash string) (*Token, error)
	FindAll(ctx context.Context) ([]Token, error)
	Revoke(ctx context.Context, id uint) error
}
