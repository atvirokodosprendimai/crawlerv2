package persistence

import "time"

type DomainModel struct {
	ID uint `gorm:"primarykey"`

	Host string `gorm:"uniqueIndex;not null"`

	// ScopeConfig
	IncludeSubdomains bool
	PathFilter        string
	MaxDepth          int
	MaxPagesPerRun    int

	// PolitenessConfig
	RespectRobotsTxt        bool
	CrawlDelayMs            int
	MaxConcurrencyPerWorker int
	MaxConcurrencyGlobal    int

	CronExpression string

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DomainModel) TableName() string { return "domains" }

type CrawlJobModel struct {
	ID       uint      `gorm:"primarykey"`
	DomainID uint      `gorm:"index;not null"`
	Status   string    `gorm:"index;not null"`
	StartedAt *time.Time
	EndedAt   *time.Time

	// ExtractConfig
	ExtractTitle   bool
	ExtractMeta    bool
	ExtractBody    bool
	DownloadBinary bool
	FilesDir       string
	MaxFileSizeMB  int

	PagesCrawled int
	CreatedAt    time.Time
}

func (CrawlJobModel) TableName() string { return "crawl_jobs" }

type URLRecordModel struct {
	ID          uint   `gorm:"primarykey"`
	DomainID    uint   `gorm:"uniqueIndex:idx_domain_url;index;not null"`
	JobID       uint   `gorm:"index;not null"`
	RawURL      string `gorm:"uniqueIndex:idx_domain_url;not null"`
	RootDomain  string `gorm:"index;not null"` // for global concurrency cap lookup
	Depth       int
	Status      string `gorm:"index;not null"`
	RetryCount  int
	FailReason  string
	LastCrawled *time.Time
	UpdatedAt   time.Time
	CreatedAt   time.Time
}

func (URLRecordModel) TableName() string { return "url_records" }

type CrawlResultModel struct {
	ID           uint   `gorm:"primarykey"`
	URLRecordID  uint   `gorm:"index;not null"`
	JobID        uint   `gorm:"index;not null"`
	StatusCode   int
	ResponseTime int64
	ContentType  string
	Title        string
	MetaDesc     string
	Body         string `gorm:"type:text"`
	FilePath     string `gorm:"index"` // object key
	FileSize     int64
	FileHash     string
	StoreID      string `gorm:"index"` // BlobStore ID
	CreatedAt    time.Time
}

func (CrawlResultModel) TableName() string { return "crawl_results" }

type TokenModel struct {
	ID        uint   `gorm:"primarykey"`
	Hash      string `gorm:"uniqueIndex;not null"`
	Revoked   bool   `gorm:"index"`
	CreatedAt time.Time
}

func (TokenModel) TableName() string { return "tokens" }
