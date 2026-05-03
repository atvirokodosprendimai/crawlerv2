package persistence

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dsn string) (*gorm.DB, error) {
	// busy_timeout lets SQLite retry writes for up to 5s when another process holds the lock
	if dsn != ":memory:" {
		dsn = dsn + "?_busy_timeout=5000"
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// WAL mode for concurrent reads
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	// Serialize writes
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&DomainModel{},
		&CrawlJobModel{},
		&URLRecordModel{},
		&CrawlResultModel{},
		&TokenModel{},
	); err != nil {
		return nil, err
	}

	return db, nil
}
