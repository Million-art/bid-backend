package database

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/platform/env"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) {
	if db == nil {
		return
	}
	_ = db.AutoMigrate(
		&domain.User{},
		&domain.ParticipantUser{},
		&domain.Product{},
		&domain.LUBParticipation{},
		&domain.BidEntry{},
		&domain.Payment{},
		&domain.RefundRecord{},
		&domain.Notification{},
		&domain.AuditLog{},
		&domain.PlatformLegalDocument{},
	)
}

func Connect(cfg *env.Config) (*gorm.DB, error) {
	dsn, err := databaseDSN(cfg)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, // winner_id, participant_id can reference client-backend IDs not in admin users
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
		sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	}

	slog.Info("connected to PostgreSQL", "max_idle_conns", 2, "max_open_conns", 10, "max_conn_max_lifetime", "30m", "max_conn_max_idle_time", "10m")
	return db, nil
}

func databaseDSN(cfg *env.Config) (string, error) {
	if cfg.DatabaseURL == "" {
		return "", fmt.Errorf("DB_URL is required")
	}

	parsed, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return "", fmt.Errorf("DB_URL is not a valid URL: %w", err)
	}

	query := parsed.Query()
	if query.Get("sslmode") == "" && cfg.DBSSLMode != "" {
		query.Set("sslmode", cfg.DBSSLMode)
	}
	if query.Get("channel_binding") == "" && cfg.DBChannelBinding != "" {
		query.Set("channel_binding", cfg.DBChannelBinding)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
