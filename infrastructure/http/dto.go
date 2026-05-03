package httpinfra

import (
	"time"

	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
)

// --- request DTOs ---

type AddDomainRequest struct {
	Host             string                  `json:"host"`
	ScopeConfig      ScopeConfigDTO          `json:"scope"`
	PolitenessConfig PolitenessConfigDTO     `json:"politeness"`
	ExtractConfig    ExtractConfigDTO        `json:"extract"`
	CronExpression   string                  `json:"cron"`
}

type ScopeConfigDTO struct {
	IncludeSubdomains bool   `json:"include_subdomains"`
	PathFilter        string `json:"path_filter"`
	MaxDepth          int    `json:"max_depth"`
	MaxPagesPerRun    int    `json:"max_pages_per_run"`
}

type PolitenessConfigDTO struct {
	RespectRobotsTxt        bool `json:"respect_robots_txt"`
	CrawlDelayMs            int  `json:"crawl_delay_ms"`
	MaxConcurrencyPerWorker int  `json:"max_concurrency_per_worker"`
	MaxConcurrencyGlobal    int  `json:"max_concurrency_global"`
}

type ExtractConfigDTO struct {
	ExtractTitle bool `json:"extract_title"`
	ExtractMeta  bool `json:"extract_meta"`
	ExtractBody  bool `json:"extract_body"`
}

type TriggerJobRequest struct {
	ExtractConfig ExtractConfigDTO `json:"extract"`
}

type SubmitResultsRequest struct {
	Results []WorkerResultItem `json:"results"`
}

type WorkerResultItem struct {
	TaskID       uint     `json:"task_id"`
	URL          string   `json:"url"`
	Depth        int      `json:"depth"`
	StatusCode   int      `json:"status_code"`
	ResponseTime int64    `json:"response_time_ms"`
	ContentType  string   `json:"content_type"`
	Title        string   `json:"title"`
	MetaDesc     string   `json:"meta_desc"`
	Body         string   `json:"body"`
	Links        []string `json:"links"`
	Error        string   `json:"error"`
}

// --- response DTOs ---

type DomainResponse struct {
	ID               uint                `json:"id"`
	Host             string              `json:"host"`
	ScopeConfig      ScopeConfigDTO      `json:"scope"`
	PolitenessConfig PolitenessConfigDTO `json:"politeness"`
	CronExpression   string              `json:"cron"`
	CreatedAt        time.Time           `json:"created_at"`
}

type JobResponse struct {
	ID           uint       `json:"id"`
	DomainID     uint       `json:"domain_id"`
	Status       string     `json:"status"`
	PagesCrawled int        `json:"pages_crawled"`
	StartedAt    *time.Time `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type TokenResponse struct {
	ID        uint      `json:"id"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTokenResponse struct {
	TokenResponse
	Token string `json:"token"` // plaintext, returned once
}

type TaskResponse struct {
	TaskID        uint                `json:"task_id"`
	URL           string              `json:"url"`
	JobID         uint                `json:"job_id"`
	Depth         int                 `json:"depth"`
	ExtractConfig ExtractConfigDTO    `json:"extract"`
	Politeness    PolitenessConfigDTO `json:"politeness"`
}

// --- mapper helpers ---

func domainToResponse(d *domain.CrawlDomain) DomainResponse {
	return DomainResponse{
		ID:   d.ID,
		Host: d.Host,
		ScopeConfig: ScopeConfigDTO{
			IncludeSubdomains: d.ScopeConfig.IncludeSubdomains,
			PathFilter:        d.ScopeConfig.PathFilter,
			MaxDepth:          d.ScopeConfig.MaxDepth,
			MaxPagesPerRun:    d.ScopeConfig.MaxPagesPerRun,
		},
		PolitenessConfig: PolitenessConfigDTO{
			RespectRobotsTxt:        d.PolitenessConfig.RespectRobotsTxt,
			CrawlDelayMs:            d.PolitenessConfig.CrawlDelayMs,
			MaxConcurrencyPerWorker: d.PolitenessConfig.MaxConcurrencyPerWorker,
			MaxConcurrencyGlobal:    d.PolitenessConfig.MaxConcurrencyGlobal,
		},
		CronExpression: d.CronExpression,
		CreatedAt:      d.CreatedAt,
	}
}

func jobToResponse(j *domain.CrawlJob) JobResponse {
	return JobResponse{
		ID:           j.ID,
		DomainID:     j.DomainID,
		Status:       string(j.Status),
		PagesCrawled: j.PagesCrawled,
		StartedAt:    j.StartedAt,
		EndedAt:      j.EndedAt,
		CreatedAt:    j.CreatedAt,
	}
}
