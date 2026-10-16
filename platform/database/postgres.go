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
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.ParticipantUser{},
		&domain.Auction{},
		&domain.AuctionParticipation{},
		&domain.Bid{},
		&domain.Payment{},
		&domain.RefundRecord{},
		&domain.Notification{},
		&domain.AuditLog{},
		&domain.PlatformLegalDocument{},
	); err != nil {
		slog.Error("database: automigrate failed", "error", err)
	}

	constraints := []string{
		// Auctions constraints
		`DO $$ BEGIN
			ALTER TABLE auctions ADD CONSTRAINT chk_auctions_entrance_fee CHECK (entrance_fee > 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE auctions ADD CONSTRAINT chk_auctions_min_bid_value CHECK (min_bid_value > 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE auctions ADD CONSTRAINT chk_auctions_max_bid_value CHECK (max_bid_value IS NULL OR max_bid_value >= min_bid_value);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE auctions ADD CONSTRAINT chk_auctions_status CHECK (status IN ('draft','active','postponed','closed','executed','cancelled'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Auction participations constraints
		`DO $$ BEGIN
			ALTER TABLE auction_participations ADD CONSTRAINT chk_participations_fee_paid CHECK (fee_paid > 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE auction_participations ADD CONSTRAINT chk_participations_status CHECK (status IN ('active','refunded'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE auction_participations ADD CONSTRAINT fk_participations_auction FOREIGN KEY (auction_id) REFERENCES auctions(id) ON UPDATE CASCADE ON DELETE CASCADE;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Auction bids constraints
		`DO $$ BEGIN
			ALTER TABLE auction_bids ADD CONSTRAINT chk_bids_bid_value CHECK (bid_value > 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE auction_bids ADD CONSTRAINT fk_bids_auction FOREIGN KEY (auction_id) REFERENCES auctions(id) ON UPDATE CASCADE ON DELETE CASCADE;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Winner bid FK on auctions (deferred after auction_bids exists)
		`DO $$ BEGIN
			ALTER TABLE auctions ADD CONSTRAINT fk_auctions_winner_bid FOREIGN KEY (winner_bid_id) REFERENCES auction_bids(id) ON UPDATE CASCADE ON DELETE SET NULL;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Payments constraints
		`DO $$ BEGIN
			ALTER TABLE payments ADD CONSTRAINT chk_payments_amount CHECK (amount >= 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE payments ADD CONSTRAINT chk_payments_status CHECK (status IN ('pending','success','failed','refunded'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE payments ADD CONSTRAINT fk_payments_auction FOREIGN KEY (auction_id) REFERENCES auctions(id) ON UPDATE CASCADE ON DELETE SET NULL;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE payments ADD CONSTRAINT fk_payments_participation FOREIGN KEY (participation_id) REFERENCES auction_participations(id) ON UPDATE CASCADE ON DELETE SET NULL;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Refund records constraints
		`DO $$ BEGIN
			ALTER TABLE refund_records ADD CONSTRAINT chk_refunds_amount CHECK (amount > 0);
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE refund_records ADD CONSTRAINT chk_refunds_status CHECK (status IN ('pending','processing','success','failed','reversed'));
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE refund_records ADD CONSTRAINT fk_refunds_auction FOREIGN KEY (auction_id) REFERENCES auctions(id) ON UPDATE CASCADE ON DELETE RESTRICT;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
		`DO $$ BEGIN
			ALTER TABLE refund_records ADD CONSTRAINT fk_refunds_participation FOREIGN KEY (participation_id) REFERENCES auction_participations(id) ON UPDATE CASCADE ON DELETE SET NULL;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,

		// Notifications FK
		`DO $$ BEGIN
			ALTER TABLE notifications ADD CONSTRAINT fk_notifications_auction FOREIGN KEY (auction_id) REFERENCES auctions(id) ON UPDATE CASCADE ON DELETE SET NULL;
		EXCEPTION WHEN duplicate_object THEN NULL; END $$;`,
	}

	for _, stmt := range constraints {
		if err := db.Exec(stmt).Error; err != nil {
			slog.Warn("database: constraint setup warning", "error", err)
		}
	}
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
		DisableForeignKeyConstraintWhenMigrating: true,
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
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
