package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type CacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	
	// Hash operations
	HGet(ctx context.Context, key, field string) (string, error)
	HSet(ctx context.Context, key string, values ...interface{}) error
	HDel(ctx context.Context, key string, fields ...string) error
	HExists(ctx context.Context, key, field string) (bool, error)
	
	// List operations
	LPush(ctx context.Context, key string, values ...interface{}) error
	RPop(ctx context.Context, key string) (string, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	
	// Set operations for tags
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)
	
	// Pub/Sub
	Publish(ctx context.Context, channel string, message interface{}) error
	Subscribe(ctx context.Context, channels ...string) <-chan *redis.Message
	
	// Health check
	Ping(ctx context.Context) error
	
	// Cache invalidation
	InvalidatePattern(ctx context.Context, pattern string) error
	InvalidateTags(ctx context.Context, tags ...string) error
}

type RedisCache struct {
	client *redis.Client
	config *CacheConfig
}

type CacheConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	MaxRetries   int
	PoolSize     int
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	DefaultTTL   time.Duration
}

type CacheItem struct {
	Data      interface{} `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
	TTL       int64       `json:"ttl"`
	Tags      []string    `json:"tags,omitempty"`
}

// Cache key patterns
const (
	ProjectCacheKey        = "project:%s"
	ServiceCacheKey        = "service:%s"
	ReleaseCacheKey        = "release:%s"
	DeploymentCacheKey     = "deployment:%s"
	UserCacheKey          = "user:%s"
	ProjectServicesCacheKey = "project:%s:services"
	ServiceReleasesCacheKey = "service:%s:releases"
	
	// Cache tags for invalidation
	ProjectTag     = "project"
	ServiceTag     = "service"
	ReleaseTag     = "release"
	DeploymentTag  = "deployment"
	UserTag        = "user"
	
	// Cache TTL
	ShortTTL  = 5 * time.Minute
	MediumTTL = 30 * time.Minute
	LongTTL   = 2 * time.Hour
	DayTTL    = 24 * time.Hour
)

func NewRedisCache(config *CacheConfig) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.DB,
		MaxRetries:   config.MaxRetries,
		PoolSize:     config.PoolSize,
		IdleTimeout:  config.IdleTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logrus.Info("Connected to Redis cache")

	return &RedisCache{
		client: rdb,
		config: config,
	}, nil
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}
	
	return []byte(val), nil
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var data []byte
	var err error
	
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
	}
	
	if ttl == 0 {
		ttl = r.config.DefaultTTL
	}
	
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

func (r *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, key, ttl).Err()
}

// Hash operations
func (r *RedisCache) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

func (r *RedisCache) HSet(ctx context.Context, key string, values ...interface{}) error {
	return r.client.HSet(ctx, key, values...).Err()
}

func (r *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

func (r *RedisCache) HExists(ctx context.Context, key, field string) (bool, error) {
	return r.client.HExists(ctx, key, field).Result()
}

// List operations
func (r *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.LPush(ctx, key, values...).Err()
}

func (r *RedisCache) RPop(ctx context.Context, key string) (string, error) {
	return r.client.RPop(ctx, key).Result()
}

func (r *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

// Set operations
func (r *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

func (r *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

func (r *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// Pub/Sub operations
func (r *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	var data []byte
	var err error
	
	switch v := message.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
	}
	
	return r.client.Publish(ctx, channel, data).Err()
}

func (r *RedisCache) Subscribe(ctx context.Context, channels ...string) <-chan *redis.Message {
	pubsub := r.client.Subscribe(ctx, channels...)
	return pubsub.Channel()
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Cache invalidation
func (r *RedisCache) InvalidatePattern(ctx context.Context, pattern string) error {
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys by pattern: %w", err)
	}
	
	if len(keys) > 0 {
		return r.Del(ctx, keys...)
	}
	
	return nil
}

func (r *RedisCache) InvalidateTags(ctx context.Context, tags ...string) error {
	for _, tag := range tags {
		tagKey := fmt.Sprintf("tag:%s", tag)
		keys, err := r.SMembers(ctx, tagKey)
		if err != nil {
			logrus.Errorf("Failed to get keys for tag %s: %v", tag, err)
			continue
		}
		
		if len(keys) > 0 {
			if err := r.Del(ctx, keys...); err != nil {
				logrus.Errorf("Failed to delete keys for tag %s: %v", tag, err)
			}
		}
		
		// Clean up the tag set
		r.Del(ctx, tagKey)
	}
	
	return nil
}

// Helper methods for caching with tags
func (r *RedisCache) SetWithTags(ctx context.Context, key string, value interface{}, ttl time.Duration, tags ...string) error {
	// Set the main cache item
	if err := r.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	
	// Add to tag sets for invalidation
	for _, tag := range tags {
		tagKey := fmt.Sprintf("tag:%s", tag)
		if err := r.SAdd(ctx, tagKey, key); err != nil {
			logrus.Errorf("Failed to add key %s to tag %s: %v", key, tag, err)
		}
	}
	
	return nil
}

// Cached operation wrapper
func (r *RedisCache) GetOrSet(ctx context.Context, key string, ttl time.Duration, fetchFunc func() (interface{}, error)) ([]byte, error) {
	// Try to get from cache first
	data, err := r.Get(ctx, key)
	if err == nil {
		return data, nil
	}
	
	if err != ErrCacheMiss {
		logrus.Errorf("Cache get error for key %s: %v", key, err)
	}
	
	// Cache miss, fetch from source
	value, err := fetchFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	
	// Store in cache
	if err := r.Set(ctx, key, value, ttl); err != nil {
		logrus.Errorf("Failed to set cache for key %s: %v", key, err)
		// Don't fail the request if cache set fails
	}
	
	// Return the data
	data, err = json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fetched data: %w", err)
	}
	
	return data, nil
}

// Close connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Errors
var (
	ErrCacheMiss = fmt.Errorf("cache miss")
)

// Default configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		MaxRetries:   3,
		PoolSize:     10,
		IdleTimeout:  300 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		DefaultTTL:   MediumTTL,
	}
}

// Cache key builders
func ProjectKey(projectID string) string {
	return fmt.Sprintf(ProjectCacheKey, projectID)
}

func ServiceKey(serviceID string) string {
	return fmt.Sprintf(ServiceCacheKey, serviceID)
}

func ReleaseKey(releaseID string) string {
	return fmt.Sprintf(ReleaseCacheKey, releaseID)
}

func DeploymentKey(deploymentID string) string {
	return fmt.Sprintf(DeploymentCacheKey, deploymentID)
}

func UserKey(userID string) string {
	return fmt.Sprintf(UserCacheKey, userID)
}

func ProjectServicesKey(projectID string) string {
	return fmt.Sprintf(ProjectServicesCacheKey, projectID)
}

func ServiceReleasesKey(serviceID string) string {
	return fmt.Sprintf(ServiceReleasesCacheKey, serviceID)
}

// Cache metrics (for monitoring)
type CacheMetrics struct {
	Hits   int64
	Misses int64
	Errors int64
}

func (r *RedisCache) GetMetrics(ctx context.Context) (*CacheMetrics, error) {
	info, err := r.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, err
	}
	
	// Parse Redis info for cache metrics
	// This is a simplified version - in production you'd parse the full stats
	return &CacheMetrics{
		Hits:   0, // Parse from info
		Misses: 0, // Parse from info
		Errors: 0, // Track in application
	}, nil
}