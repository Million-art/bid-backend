// Seed admin user from backend/.env
package main

import (
    "os"
    "log/slog"

    "github.com/bid-hub/admin-backend/adapters/repository/postgres"
    "github.com/bid-hub/admin-backend/core/domain"
    "github.com/bid-hub/admin-backend/platform/database"
    "github.com/bid-hub/admin-backend/platform/env"
    "github.com/bid-hub/admin-backend/platform/logger"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
    "gorm.io/gorm"
)

func main() {
    // Configure the process-wide structured logger from env (LOG_LEVEL, LOG_FORMAT).
    cfg := env.LoadConfig()
    slog.SetDefault(logger.New(cfg.LogLevel, cfg.LogFormat))

    if err := cfg.Validate(); err != nil {
        slog.Error("Configuration validation failed", "error", err)
        os.Exit(1)
    }

    // Connect to Database
    db, err := database.Connect(cfg)
    if err != nil {
        slog.Error("Failed to connect to database", "error", err)
        os.Exit(1)
    }

    database.Migrate(db)

    // Seed admin user
    seedAdmin(cfg, db)
    slog.Info("Admin seeding completed successfully")
}

func seedAdmin(cfg *env.Config, db *gorm.DB) {
    // Check if admin user already exists
    admin, err := postgres.NewUserRepository(db).GetByEmail(cfg.SeedAdminEmail)
    if err == nil && admin != nil {
        slog.Info("Admin user already exists, skipping seed", "email", cfg.SeedAdminEmail)
        return
    }

    // Create admin user
    hash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedAdminPassword), bcrypt.DefaultCost)
    if err != nil {
        slog.Error("Failed to hash admin password", "error", err)
        os.Exit(1)
    }

    user := &domain.User{
        ID:       uuid.New(),
        FullName: "System Admin",
        Email:    cfg.SeedAdminEmail,
        Password: string(hash),
        Role:     domain.RoleSuperAdmin,
        Status:   domain.StatusActive,
    }

    if err := postgres.NewUserRepository(db).Create(user); err != nil {
        slog.Error("Failed to seed admin user", "error", err)
        os.Exit(1)
    }

    slog.Info("Admin user seeded successfully", "email", cfg.SeedAdminEmail)
}
