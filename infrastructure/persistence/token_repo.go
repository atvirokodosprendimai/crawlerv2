package persistence

import (
	"context"

	"github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"gorm.io/gorm"
)

type GormTokenRepository struct{ db *gorm.DB }

func NewTokenRepository(db *gorm.DB) *GormTokenRepository {
	return &GormTokenRepository{db: db}
}

func (r *GormTokenRepository) Save(ctx context.Context, t *crawler.Token) error {
	m := TokenModel{Hash: t.Hash}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		return err
	}
	t.ID = m.ID
	t.CreatedAt = m.CreatedAt
	return nil
}

func (r *GormTokenRepository) FindByHash(ctx context.Context, hash string) (*crawler.Token, error) {
	var m TokenModel
	if err := r.db.WithContext(ctx).Where("hash = ? AND revoked = false", hash).First(&m).Error; err != nil {
		return nil, err
	}
	tok := crawler.Token{ID: m.ID, Hash: m.Hash, Revoked: m.Revoked, CreatedAt: m.CreatedAt}
	return &tok, nil
}

func (r *GormTokenRepository) FindAll(ctx context.Context) ([]crawler.Token, error) {
	var models []TokenModel
	if err := r.db.WithContext(ctx).Find(&models).Error; err != nil {
		return nil, err
	}
	out := make([]crawler.Token, len(models))
	for i, m := range models {
		out[i] = crawler.Token{ID: m.ID, Hash: m.Hash, Revoked: m.Revoked, CreatedAt: m.CreatedAt}
	}
	return out, nil
}

func (r *GormTokenRepository) Revoke(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&TokenModel{}).Where("id = ?", id).
		Update("revoked", true).Error
}
