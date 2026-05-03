package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"gorm.io/gorm"
)

type GormCrawlResultRepository struct{ db *gorm.DB }

func NewCrawlResultRepository(db *gorm.DB) *GormCrawlResultRepository {
	return &GormCrawlResultRepository{db: db}
}

func (r *GormCrawlResultRepository) Save(ctx context.Context, result *crawler.CrawlResult) error {
	m := resultToModel(result)
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	result.ID = m.ID
	return nil
}

func (r *GormCrawlResultRepository) FindByJob(ctx context.Context, jobID uint) ([]crawler.CrawlResult, error) {
	var models []CrawlResultModel
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]crawler.CrawlResult, len(models))
	for i, m := range models {
		out[i] = modelToResult(&m)
	}
	return out, nil
}

func (r *GormCrawlResultRepository) FindByStore(ctx context.Context, storeID string) ([]crawler.CrawlResult, error) {
	var models []CrawlResultModel
	if err := r.db.WithContext(ctx).Where("store_id = ? AND file_path != ''", storeID).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]crawler.CrawlResult, len(models))
	for i, m := range models {
		out[i] = modelToResult(&m)
	}
	return out, nil
}

func (r *GormCrawlResultRepository) UpdateStore(ctx context.Context, resultID uint, newStoreID string) error {
	return r.db.WithContext(ctx).Model(&CrawlResultModel{}).Where("id = ?", resultID).
		Update("store_id", newStoreID).Error
}

func resultToModel(r *crawler.CrawlResult) CrawlResultModel {
	return CrawlResultModel{
		ID:           r.ID,
		URLRecordID:  r.URLRecordID,
		JobID:        r.JobID,
		StatusCode:   r.StatusCode,
		ResponseTime: r.ResponseTime,
		ContentType:  r.ContentType,
		Title:        r.Title,
		MetaDesc:     r.MetaDesc,
		Body:         r.Body,
		FilePath:     r.FilePath,
		FileSize:     r.FileSize,
		FileHash:     r.FileHash,
		StoreID:      r.StoreID,
	}
}

func modelToResult(m *CrawlResultModel) crawler.CrawlResult {
	return crawler.CrawlResult{
		ID:           m.ID,
		URLRecordID:  m.URLRecordID,
		JobID:        m.JobID,
		StatusCode:   m.StatusCode,
		ResponseTime: m.ResponseTime,
		ContentType:  m.ContentType,
		Title:        m.Title,
		MetaDesc:     m.MetaDesc,
		Body:         m.Body,
		FilePath:     m.FilePath,
		FileSize:     m.FileSize,
		FileHash:     m.FileHash,
		StoreID:      m.StoreID,
		CreatedAt:    m.CreatedAt,
	}
}
