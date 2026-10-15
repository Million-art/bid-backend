package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage implements Fiber's Storage interface using go-redis
type RedisStorage struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisStorage(client *redis.Client) *RedisStorage {
	return &RedisStorage{
		client: client,
		ctx:    context.Background(),
	}
}

func (s *RedisStorage) Get(key string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	return s.GetWithContext(s.ctx, key)
}

func (s *RedisStorage) Set(key string, val []byte, exp time.Duration) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.SetWithContext(s.ctx, key, val, exp)
}

func (s *RedisStorage) Delete(key string) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.DeleteWithContext(s.ctx, key)
}

func (s *RedisStorage) Reset() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.ResetWithContext(s.ctx)
}

func (s *RedisStorage) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *RedisStorage) GetWithContext(ctx context.Context, key string) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	val, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // fiber expects nil for missing key
	}
	return val, err
}

func (s *RedisStorage) SetWithContext(ctx context.Context, key string, val []byte, exp time.Duration) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Set(ctx, key, val, exp).Err()
}

func (s *RedisStorage) DeleteWithContext(ctx context.Context, key string) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Del(ctx, key).Err()
}

func (s *RedisStorage) ResetWithContext(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.FlushDB(ctx).Err()
}

