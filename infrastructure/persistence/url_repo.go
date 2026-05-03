package persistence

import (
	"context"
	"net/url"
	"time"

	"github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func parseHost(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

type GormURLRepository struct{ db *gorm.DB }

func NewURLRepository(db *gorm.DB) *GormURLRepository {
	return &GormURLRepository{db: db}
}

func (r *GormURLRepository) Enqueue(ctx context.Context, urls []crawler.URLRecord) error {
	if len(urls) == 0 {
		return nil
	}
	models := make([]URLRecordModel, len(urls))
	for i, u := range urls {
		models[i] = urlToModel(&u)
	}
	// Insert only if (domain_id, raw_url) not already present
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "domain_id"}, {Name: "raw_url"}},
			DoNothing: true,
		}).
		Create(&models).Error
}

func (r *GormURLRepository) ClaimBatch(ctx context.Context, jobID uint, batchSize int) ([]crawler.URLRecord, error) {
	var models []URLRecordModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ? AND status = ?", jobID, string(crawler.URLStatusPending)).
			Order("created_at ASC").
			Limit(batchSize).
			Find(&models).Error; err != nil {
			return err
		}
		if len(models) == 0 {
			return nil
		}
		ids := make([]uint, len(models))
		for i, m := range models {
			ids[i] = m.ID
		}
		return tx.Model(&URLRecordModel{}).Where("id IN ?", ids).
			Updates(map[string]any{
				"status":     string(crawler.URLStatusInProgress),
				"updated_at": time.Now(),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]crawler.URLRecord, len(models))
	for i, m := range models {
		out[i] = modelToURL(&m)
	}
	return out, nil
}

func (r *GormURLRepository) MarkDone(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&URLRecordModel{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":       string(crawler.URLStatusDone),
			"last_crawled": &now,
			"updated_at":   now,
		}).Error
}

func (r *GormURLRepository) MarkFailed(ctx context.Context, id uint, reason string) error {
	return r.db.WithContext(ctx).Model(&URLRecordModel{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":      string(crawler.URLStatusFailed),
			"fail_reason": reason,
			"retry_count": gorm.Expr("retry_count + 1"),
			"updated_at":  time.Now(),
		}).Error
}

func (r *GormURLRepository) ReclaimStale(ctx context.Context, threshold time.Duration) (int64, error) {
	cutoff := time.Now().Add(-threshold)
	res := r.db.WithContext(ctx).Model(&URLRecordModel{}).
		Where("status = ? AND updated_at < ?", string(crawler.URLStatusInProgress), cutoff).
		Updates(map[string]any{
			"status":     string(crawler.URLStatusPending),
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

func (r *GormURLRepository) CountInProgress(ctx context.Context, rootDomain string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&URLRecordModel{}).
		Where("root_domain = ? AND status = ?", rootDomain, string(crawler.URLStatusInProgress)).
		Count(&count).Error
	return count, err
}

func (r *GormURLRepository) CountDone(ctx context.Context, jobID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&URLRecordModel{}).
		Where("job_id = ? AND status = ?", jobID, string(crawler.URLStatusDone)).
		Count(&count).Error
	return count, err
}

func urlToModel(u *crawler.URLRecord) URLRecordModel {
	host := u.RawURL
	if parsed, err := parseHost(u.RawURL); err == nil {
		host = parsed
	}
	return URLRecordModel{
		ID:          u.ID,
		DomainID:    u.DomainID,
		JobID:       u.JobID,
		RawURL:      u.RawURL,
		RootDomain:  crawler.RootDomain(host),
		Depth:       u.Depth,
		Status:      string(u.Status),
		RetryCount:  u.RetryCount,
		LastCrawled: u.LastCrawled,
	}
}

func modelToURL(m *URLRecordModel) crawler.URLRecord {
	return crawler.URLRecord{
		ID:          m.ID,
		DomainID:    m.DomainID,
		JobID:       m.JobID,
		RawURL:      m.RawURL,
		Depth:       m.Depth,
		Status:      crawler.URLStatus(m.Status),
		RetryCount:  m.RetryCount,
		LastCrawled: m.LastCrawled,
		UpdatedAt:   m.UpdatedAt,
		CreatedAt:   m.CreatedAt,
	}
}
