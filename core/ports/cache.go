package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CacheRepository interface {
	GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, bool, error)
	SetUnreadCount(ctx context.Context, userID uuid.UUID, count int64) error
	InvalidateUnreadCount(ctx context.Context, userID uuid.UUID) error

	// AcquireLock attempts to set key with a TTL using SET NX (acquire a distributed lock).
	// Returns true if the lock was acquired, false if it was already held.
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)

	// ReleaseLock deletes the lock key, releasing the distributed lock.
	ReleaseLock(ctx context.Context, key string) error

	// RevokeToken adds a token JTI to the revocation set with the given TTL (should match
	// the token's remaining lifetime so the entry expires naturally).
	RevokeToken(ctx context.Context, jti string, ttl time.Duration) error

	// IsTokenRevoked returns true if the given JTI has been revoked.
	IsTokenRevoked(ctx context.Context, jti string) (bool, error)

	// OTP Methods
	SetOTP(ctx context.Context, phone string, code string, ttl time.Duration) error
	GetOTP(ctx context.Context, phone string) (string, error)
	DeleteOTP(ctx context.Context, phone string) error

	// Ping checks Redis connectivity
	Ping(ctx context.Context) error
}
