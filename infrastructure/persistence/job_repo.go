package persistence

import (
	"context"
	"time"

	"github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"gorm.io/gorm"
)

type GormCrawlJobRepository struct{ db *gorm.DB }

func NewCrawlJobRepository(db *gorm.DB) *GormCrawlJobRepository {
	return &GormCrawlJobRepository{db: db}
}

func (r *GormCrawlJobRepository) Create(ctx context.Context, job *crawler.CrawlJob) error {
	m := jobToModel(job)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	job.ID = m.ID
	return nil
}

func (r *GormCrawlJobRepository) UpdateStatus(ctx context.Context, jobID uint, status crawler.JobStatus) error {
	return r.db.WithContext(ctx).Model(&CrawlJobModel{}).Where("id = ?", jobID).
		Update("status", string(status)).Error
}

func (r *GormCrawlJobRepository) IncrementPages(ctx context.Context, jobID uint, count int) error {
	return r.db.WithContext(ctx).Model(&CrawlJobModel{}).Where("id = ?", jobID).
		UpdateColumn("pages_crawled", gorm.Expr("pages_crawled + ?", count)).Error
}

func (r *GormCrawlJobRepository) FindByID(ctx context.Context, id uint) (*crawler.CrawlJob, error) {
	var m CrawlJobModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	j := modelToJob(&m)
	return &j, nil
}

func (r *GormCrawlJobRepository) FindActive(ctx context.Context, domainID uint) (*crawler.CrawlJob, error) {
	var m CrawlJobModel
	err := r.db.WithContext(ctx).
		Where("domain_id = ? AND status IN ?", domainID, []string{"pending", "running"}).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	j := modelToJob(&m)
	return &j, nil
}

func (r *GormCrawlJobRepository) FindAll(ctx context.Context, domainID uint, status crawler.JobStatus) ([]crawler.CrawlJob, error) {
	q := r.db.WithContext(ctx).Model(&CrawlJobModel{})
	if domainID > 0 {
		q = q.Where("domain_id = ?", domainID)
	}
	if status != "" {
		q = q.Where("status = ?", string(status))
	}
	var models []CrawlJobModel
	if err := q.Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]crawler.CrawlJob, len(models))
	for i, m := range models {
		out[i] = modelToJob(&m)
	}
	return out, nil
}

func (r *GormCrawlJobRepository) SetEnded(ctx context.Context, jobID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&CrawlJobModel{}).Where("id = ?", jobID).
		Update("ended_at", &now).Error
}

func jobToModel(j *crawler.CrawlJob) CrawlJobModel {
	return CrawlJobModel{
		ID:           j.ID,
		DomainID:     j.DomainID,
		Status:       string(j.Status),
		StartedAt:    j.StartedAt,
		EndedAt:      j.EndedAt,
		ExtractTitle: j.ExtractConfig.ExtractTitle,
		ExtractMeta:  j.ExtractConfig.ExtractMeta,
		ExtractBody:  j.ExtractConfig.ExtractBody,
		PagesCrawled: j.PagesCrawled,
	}
}

func modelToJob(m *CrawlJobModel) crawler.CrawlJob {
	return crawler.CrawlJob{
		ID:       m.ID,
		DomainID: m.DomainID,
		Status:   crawler.JobStatus(m.Status),
		ExtractConfig: crawler.ExtractConfig{
			ExtractTitle: m.ExtractTitle,
			ExtractMeta:  m.ExtractMeta,
			ExtractBody:  m.ExtractBody,
		},
		PagesCrawled: m.PagesCrawled,
		StartedAt:    m.StartedAt,
		EndedAt:      m.EndedAt,
		CreatedAt:    m.CreatedAt,
	}
}
