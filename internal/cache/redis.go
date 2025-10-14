package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis wraps a go-redis client with logging helpers.
type Redis struct {
	client *redis.Client
	logger *slog.Logger
}

// Config defines connection parameters for Redis.
type Config struct {
	Addr     string
	Password string
	DB       int
	UseTLS   bool
}

// New returns a Redis client based on provided configuration.
func New(cfg Config, logger *slog.Logger) *Redis {
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.UseTLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &Redis{
		client: redis.NewClient(opts),
		logger: logger.With("component", "redis"),
	}
}

// Client exposes the underlying go-redis client.
func (r *Redis) Client() *redis.Client {
	return r.client
}

// Ping verifies Redis connectivity.
func (r *Redis) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// SetJSON caches a value as JSON with the provided TTL.
func (r *Redis) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := jsonMarshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

// GetJSON retrieves JSON value and unmarshals into dest.
func (r *Redis) GetJSON(ctx context.Context, key string, dest any) (bool, error) {
	res, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("redis get %s: %w", key, err)
	}
	if err := jsonUnmarshal([]byte(res), dest); err != nil {
		return false, err
	}
	return true, nil
}

// Close releases Redis resources.
func (r *Redis) Close() error {
	return r.client.Close()
}

func jsonMarshal(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}
	return data, nil
}

func jsonUnmarshal(data []byte, dest any) error {
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}
	return nil
}
