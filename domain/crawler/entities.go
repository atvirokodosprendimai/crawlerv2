package crawler

import "time"

type CrawlDomain struct {
	ID               uint
	Host             string
	ScopeConfig      ScopeConfig
	PolitenessConfig PolitenessConfig
	CronExpression   string // empty = manual only
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CrawlJob struct {
	ID            uint
	DomainID      uint
	Status        JobStatus
	ExtractConfig ExtractConfig
	PagesCrawled  int
	StartedAt     *time.Time
	EndedAt       *time.Time
	CreatedAt     time.Time
}

type URLRecord struct {
	ID          uint
	DomainID    uint
	JobID       uint
	RawURL      string
	Depth       int
	Status      URLStatus
	RetryCount  int
	LastCrawled *time.Time
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

type CrawlResult struct {
	ID           uint
	URLRecordID  uint
	JobID        uint
	StatusCode   int
	ResponseTime int64 // ms
	ContentType  string
	Title        string
	MetaDesc     string
	Body         string
	CreatedAt    time.Time
}

type Token struct {
	ID        uint
	Hash      string // bcrypt hash of the plaintext token
	Revoked   bool
	CreatedAt time.Time
}

// OutboundLink is a discovered link from a crawled page.
type OutboundLink struct {
	URL   string
	Depth int
}
