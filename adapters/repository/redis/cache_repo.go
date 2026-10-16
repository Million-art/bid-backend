package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Compile-time check that CacheRepo satisfies the interface.
var _ ports.CacheRepository = (*CacheRepo)(nil)

type CacheRepo struct {
	client *redis.Client

	lockTokens sync.Map // map[string]string (key -> token)
}

func NewCacheRepository(client *redis.Client) ports.CacheRepository {
	return &CacheRepo{client: client}
}

const (
	defaultOpTimeout = 2 * time.Second
	keyNamespace     = "admin"
)

func (r *CacheRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, defaultOpTimeout)
}

func (r *CacheRepo) key(parts ...string) string {
	clean := make([]string, 0, len(parts)+1)
	clean = append(clean, keyNamespace)
	for _, p := range parts {
		p = strings.Trim(p, ":")
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, ":")
}

func (r *CacheRepo) getUnreadCountKey(userID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", r.key("unread_count"), userID.String())
}

func (r *CacheRepo) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, bool, error) {
	if r == nil || r.client == nil {
		return 0, false, fmt.Errorf("redis unavailable")
	}

	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	val, err := r.client.Get(opCtx, r.getUnreadCountKey(userID)).Int64()
	if err == redis.Nil {
		return 0, false, nil
	} else if err != nil {
		return 0, false, err
	}

	return val, true, nil
}

func (r *CacheRepo) SetUnreadCount(ctx context.Context, userID uuid.UUID, count int64) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("redis unavailable")
	}

	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Cache for 1 hour
	return r.client.Set(opCtx, r.getUnreadCountKey(userID), count, 1*time.Hour).Err()
}

func (r *CacheRepo) InvalidateUnreadCount(ctx context.Context, userID uuid.UUID) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("redis unavailable")
	}

	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.client.Del(opCtx, r.getUnreadCountKey(userID)).Err()
}

// AcquireLock attempts a SET NX (set-if-not-exists) on key with the given TTL.
// Returns true when the lock is acquired, false when it is already held by another caller.
func (r *CacheRepo) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil {
		return false, fmt.Errorf("redis unavailable")
	}

	lockKey := r.key("lock", key)
	token := uuid.NewString()

	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.client.Do(opCtx, "SET", lockKey, token, "EX", int(ttl.Seconds()), "NX").Result()
	ok := err == nil && res != nil
	if err != nil {
		return false, err
	}
	if ok {
		r.lockTokens.Store(lockKey, token)
	}
	return ok, nil
}

// ReleaseLock deletes the distributed lock key.
func (r *CacheRepo) ReleaseLock(ctx context.Context, key string) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("redis unavailable")
	}

	lockKey := r.key("lock", key)
	tokenAny, ok := r.lockTokens.Load(lockKey)
	if !ok {
		return nil
	}
	token, _ := tokenAny.(string)
	if token == "" {
		r.lockTokens.Delete(lockKey)
		return nil
	}

	const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
`
	script := redis.NewScript(releaseScript)

	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := script.Run(opCtx, r.client, []string{lockKey}, token).Result() //nolint:staticcheck // nolint:staticcheck
	r.lockTokens.Delete(lockKey)
	return err
}

func (r *CacheRepo) getRevokedTokenKey(jti string) string {
	return r.key("revoked_token", jti)
}

// RevokeToken stores the JTI in Redis with the given TTL so it is automatically
// cleaned up once the token would have expired anyway.
func (r *CacheRepo) RevokeToken(ctx context.Context, jti string, ttl time.Duration) error {
	if r == nil || r.client == nil {
		if r != nil {
			r.lockTokens.Store("revoked:"+jti, true)
		}
		return nil
	}
	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.client.Set(opCtx, r.getRevokedTokenKey(jti), "1", ttl).Err()
}

// IsTokenRevoked returns true when the JTI is present in the revocation set.
func (r *CacheRepo) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	if r == nil || r.client == nil {
		if r != nil {
			_, ok := r.lockTokens.Load("revoked:" + jti)
			return ok, nil
		}
		return false, nil
	}

	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	exists, err := r.client.Exists(opCtx, r.getRevokedTokenKey(jti)).Result() //nolint:staticcheck // nolint:staticcheck
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (r *CacheRepo) getOTPKey(phone string) string {
	return r.key("otp", phone)
}

func (r *CacheRepo) SetOTP(ctx context.Context, phone string, code string, ttl time.Duration) error {
	if r == nil || r.client == nil {
		r.lockTokens.Store("otp:"+phone, code)
		return nil
	}
	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.client.Set(opCtx, r.getOTPKey(phone), code, ttl).Err()
}

func (r *CacheRepo) GetOTP(ctx context.Context, phone string) (string, error) {
	if r == nil || r.client == nil {
		if val, ok := r.lockTokens.Load("otp:" + phone); ok {
			return val.(string), nil
		}
		return "", nil
	}
	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	val, err := r.client.Get(opCtx, r.getOTPKey(phone)).Result() //nolint:staticcheck // nolint:staticcheck
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *CacheRepo) DeleteOTP(ctx context.Context, phone string) error {
	if r == nil || r.client == nil {
		r.lockTokens.Delete("otp:" + phone)
		return nil
	}
	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.client.Del(opCtx, r.getOTPKey(phone)).Err()
}

func (r *CacheRepo) Ping(ctx context.Context) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	opCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	return r.client.Ping(opCtx).Err()
}
