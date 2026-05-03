package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormDomainRepository struct{ db *gorm.DB }

func NewDomainRepository(db *gorm.DB) *GormDomainRepository {
	return &GormDomainRepository{db: db}
}

func (r *GormDomainRepository) Save(ctx context.Context, d *crawler.CrawlDomain) error {
	m := domainToModel(d)
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Save(&m)
	if res.Error != nil {
		return res.Error
	}
	d.ID = m.ID
	return nil
}

func (r *GormDomainRepository) FindByID(ctx context.Context, id uint) (*crawler.CrawlDomain, error) {
	var m DomainModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	d := modelToDomain(&m)
	return &d, nil
}

func (r *GormDomainRepository) FindAll(ctx context.Context) ([]crawler.CrawlDomain, error) {
	var models []DomainModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]crawler.CrawlDomain, len(models))
	for i, m := range models {
		out[i] = modelToDomain(&m)
	}
	return out, nil
}

func (r *GormDomainRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&DomainModel{}, id).Error
}

func domainToModel(d *crawler.CrawlDomain) DomainModel {
	return DomainModel{
		ID:                      d.ID,
		Host:                    d.Host,
		IncludeSubdomains:       d.ScopeConfig.IncludeSubdomains,
		PathFilter:              d.ScopeConfig.PathFilter,
		MaxDepth:                d.ScopeConfig.MaxDepth,
		MaxPagesPerRun:          d.ScopeConfig.MaxPagesPerRun,
		RespectRobotsTxt:        d.PolitenessConfig.RespectRobotsTxt,
		CrawlDelayMs:            d.PolitenessConfig.CrawlDelayMs,
		MaxConcurrencyPerWorker: d.PolitenessConfig.MaxConcurrencyPerWorker,
		MaxConcurrencyGlobal:    d.PolitenessConfig.MaxConcurrencyGlobal,
		CronExpression:          d.CronExpression,
	}
}

func modelToDomain(m *DomainModel) crawler.CrawlDomain {
	return crawler.CrawlDomain{
		ID:   m.ID,
		Host: m.Host,
		ScopeConfig: crawler.ScopeConfig{
			IncludeSubdomains: m.IncludeSubdomains,
			PathFilter:        m.PathFilter,
			MaxDepth:          m.MaxDepth,
			MaxPagesPerRun:    m.MaxPagesPerRun,
		},
		PolitenessConfig: crawler.PolitenessConfig{
			RespectRobotsTxt:        m.RespectRobotsTxt,
			CrawlDelayMs:            m.CrawlDelayMs,
			MaxConcurrencyPerWorker: m.MaxConcurrencyPerWorker,
			MaxConcurrencyGlobal:    m.MaxConcurrencyGlobal,
		},
		CronExpression: m.CronExpression,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
