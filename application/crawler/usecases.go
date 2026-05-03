package crawler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrJobAlreadyRunning = errors.New("a job is already running for this domain")
	ErrDomainNotFound    = errors.New("domain not found")
	ErrJobNotFound       = errors.New("job not found")
)

// AddDomainInput carries everything needed to register a new crawl domain.
type AddDomainInput struct {
	Host             string
	ScopeConfig      domain.ScopeConfig
	PolitenessConfig domain.PolitenessConfig
	CronExpression   string
	ExtractConfig    domain.ExtractConfig
}

type AddDomainUseCase struct {
	domains domain.DomainRepository
	urls    domain.URLRepository
	jobs    domain.CrawlJobRepository
}

func NewAddDomainUseCase(d domain.DomainRepository, u domain.URLRepository, j domain.CrawlJobRepository) *AddDomainUseCase {
	return &AddDomainUseCase{domains: d, urls: u, jobs: j}
}

func (uc *AddDomainUseCase) Execute(ctx context.Context, in AddDomainInput) (*domain.CrawlDomain, error) {
	d := &domain.CrawlDomain{
		Host:             in.Host,
		ScopeConfig:      in.ScopeConfig,
		PolitenessConfig: in.PolitenessConfig,
		CronExpression:   in.CronExpression,
	}
	if err := uc.domains.Save(ctx, d); err != nil {
		return nil, fmt.Errorf("save domain: %w", err)
	}
	return d, nil
}

// UpdateDomainUseCase updates scope/politeness/schedule for an existing domain.
type UpdateDomainUseCase struct {
	domains domain.DomainRepository
}

func NewUpdateDomainUseCase(d domain.DomainRepository) *UpdateDomainUseCase {
	return &UpdateDomainUseCase{domains: d}
}

func (uc *UpdateDomainUseCase) Execute(ctx context.Context, id uint, in AddDomainInput) (*domain.CrawlDomain, error) {
	d, err := uc.domains.FindByID(ctx, id)
	if err != nil {
		return nil, ErrDomainNotFound
	}
	d.Host = in.Host
	d.ScopeConfig = in.ScopeConfig
	d.PolitenessConfig = in.PolitenessConfig
	d.CronExpression = in.CronExpression
	if err := uc.domains.Save(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// TriggerJobInput carries job trigger parameters.
type TriggerJobInput struct {
	DomainID      uint
	ExtractConfig domain.ExtractConfig
}

type TriggerJobUseCase struct {
	domains domain.DomainRepository
	jobs    domain.CrawlJobRepository
	urls    domain.URLRepository
}

func NewTriggerJobUseCase(d domain.DomainRepository, j domain.CrawlJobRepository, u domain.URLRepository) *TriggerJobUseCase {
	return &TriggerJobUseCase{domains: d, jobs: j, urls: u}
}

func (uc *TriggerJobUseCase) Execute(ctx context.Context, in TriggerJobInput) (*domain.CrawlJob, error) {
	d, err := uc.domains.FindByID(ctx, in.DomainID)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	existing, err := uc.jobs.FindActive(ctx, in.DomainID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check active job: %w", err)
	}
	if existing != nil {
		return nil, ErrJobAlreadyRunning
	}

	now := time.Now()
	job := &domain.CrawlJob{
		DomainID:      in.DomainID,
		Status:        domain.JobStatusRunning,
		ExtractConfig: in.ExtractConfig,
		StartedAt:     &now,
	}
	if err := uc.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	// Seed the root URL into the queue
	seedURL := "https://" + d.Host
	if norm, err := domain.NormalizeURL(seedURL); err == nil {
		seedURL = norm
	}
	_ = uc.urls.Enqueue(ctx, []domain.URLRecord{{
		DomainID: d.ID,
		JobID:    job.ID,
		RawURL:   seedURL,
		Depth:    0,
		Status:   domain.URLStatusPending,
	}})

	return job, nil
}

// PollTasksInput carries worker poll parameters.
type PollTasksInput struct {
	JobID     uint
	BatchSize int
}

type TaskDTO struct {
	TaskID        uint
	URL           string
	JobID         uint
	Depth         int
	ExtractConfig domain.ExtractConfig
	Politeness    domain.PolitenessConfig
}

type PollTasksUseCase struct {
	jobs    domain.CrawlJobRepository
	domains domain.DomainRepository
	urls    domain.URLRepository
}

func NewPollTasksUseCase(j domain.CrawlJobRepository, d domain.DomainRepository, u domain.URLRepository) *PollTasksUseCase {
	return &PollTasksUseCase{jobs: j, domains: d, urls: u}
}

func (uc *PollTasksUseCase) Execute(ctx context.Context, in PollTasksInput) ([]TaskDTO, error) {
	job, err := uc.jobs.FindByID(ctx, in.JobID)
	if err != nil {
		return nil, ErrJobNotFound
	}
	d, err := uc.domains.FindByID(ctx, job.DomainID)
	if err != nil {
		return nil, ErrDomainNotFound
	}

	batchSize := in.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	// Enforce global concurrency cap
	if d.PolitenessConfig.MaxConcurrencyGlobal > 0 {
		rootDom := domain.RootDomain(d.Host)
		inFlight, err := uc.urls.CountInProgress(ctx, rootDom)
		if err != nil {
			return nil, err
		}
		available := int64(d.PolitenessConfig.MaxConcurrencyGlobal) - inFlight
		if available <= 0 {
			return nil, nil
		}
		if int64(batchSize) > available {
			batchSize = int(available)
		}
	}

	records, err := uc.urls.ClaimBatch(ctx, in.JobID, batchSize)
	if err != nil {
		return nil, err
	}

	tasks := make([]TaskDTO, len(records))
	for i, r := range records {
		tasks[i] = TaskDTO{
			TaskID:        r.ID,
			URL:           r.RawURL,
			JobID:         r.JobID,
			Depth:         r.Depth,
			ExtractConfig: job.ExtractConfig,
			Politeness:    d.PolitenessConfig,
		}
	}
	return tasks, nil
}

// SubmitResultInput is one crawled URL result with its discovered links.
type SubmitResultInput struct {
	TaskID       uint
	URL          string
	Depth        int
	StatusCode   int
	ResponseTime int64
	ContentType  string
	Title        string
	MetaDesc     string
	Body         string
	Links        []string
	Error        string
}

type SubmitResultsUseCase struct {
	jobs    domain.CrawlJobRepository
	domains domain.DomainRepository
	urls    domain.URLRepository
	results domain.CrawlResultRepository
}

func NewSubmitResultsUseCase(
	j domain.CrawlJobRepository,
	d domain.DomainRepository,
	u domain.URLRepository,
	r domain.CrawlResultRepository,
) *SubmitResultsUseCase {
	return &SubmitResultsUseCase{jobs: j, domains: d, urls: u, results: r}
}

func (uc *SubmitResultsUseCase) Execute(ctx context.Context, jobID uint, inputs []SubmitResultInput) error {
	job, err := uc.jobs.FindByID(ctx, jobID)
	if err != nil {
		return ErrJobNotFound
	}
	d, err := uc.domains.FindByID(ctx, job.DomainID)
	if err != nil {
		return ErrDomainNotFound
	}

	var toEnqueue []domain.URLRecord
	doneCount := 0

	for _, inp := range inputs {
		if inp.Error != "" {
			_ = uc.urls.MarkFailed(ctx, inp.TaskID, inp.Error)
			continue
		}

		_ = uc.urls.MarkDone(ctx, inp.TaskID)
		doneCount++

		result := &domain.CrawlResult{
			URLRecordID:  inp.TaskID,
			JobID:        jobID,
			StatusCode:   inp.StatusCode,
			ResponseTime: inp.ResponseTime,
			ContentType:  inp.ContentType,
			Title:        inp.Title,
			MetaDesc:     inp.MetaDesc,
		}
		if job.ExtractConfig.ExtractBody {
			result.Body = inp.Body
		}
		_ = uc.results.Save(ctx, result)

		// Discover new URLs
		for _, link := range inp.Links {
			norm, err := domain.NormalizeURL(link)
			if err != nil {
				continue
			}
			nextDepth := inp.Depth + 1
			if !domain.URLInScope(norm, d.Host, d.ScopeConfig, nextDepth) {
				continue
			}
			// Check max_pages_per_run
			if d.ScopeConfig.MaxPagesPerRun > 0 && job.PagesCrawled+doneCount+len(toEnqueue) >= d.ScopeConfig.MaxPagesPerRun {
				break
			}
			toEnqueue = append(toEnqueue, domain.URLRecord{
				DomainID: d.ID,
				JobID:    jobID,
				RawURL:   norm,
				Depth:    nextDepth,
				Status:   domain.URLStatusPending,
			})
		}
	}

	if len(toEnqueue) > 0 {
		_ = uc.urls.Enqueue(ctx, toEnqueue)
	}
	if doneCount > 0 {
		_ = uc.jobs.IncrementPages(ctx, jobID, doneCount)
	}

	return nil
}

// ManageTokenUseCase creates and revokes bearer tokens.
type ManageTokenUseCase struct {
	tokens domain.TokenRepository
}

func NewManageTokenUseCase(t domain.TokenRepository) *ManageTokenUseCase {
	return &ManageTokenUseCase{tokens: t}
}

func (uc *ManageTokenUseCase) Create(ctx context.Context) (plaintext string, tok *domain.Token, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, err
	}
	plaintext = hex.EncodeToString(raw)
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}
	tok = &domain.Token{Hash: string(hash)}
	if err = uc.tokens.Save(ctx, tok); err != nil {
		return "", nil, err
	}
	return plaintext, tok, nil
}

func (uc *ManageTokenUseCase) Revoke(ctx context.Context, id uint) error {
	return uc.tokens.Revoke(ctx, id)
}

func (uc *ManageTokenUseCase) List(ctx context.Context) ([]domain.Token, error) {
	return uc.tokens.FindAll(ctx)
}

// ReclaimStaleTasksUseCase resets stale in-progress URL records.
type ReclaimStaleTasksUseCase struct {
	urls domain.URLRepository
}

func NewReclaimStaleTasksUseCase(u domain.URLRepository) *ReclaimStaleTasksUseCase {
	return &ReclaimStaleTasksUseCase{urls: u}
}

func (uc *ReclaimStaleTasksUseCase) Execute(ctx context.Context, threshold time.Duration) (int64, error) {
	return uc.urls.ReclaimStale(ctx, threshold)
}
