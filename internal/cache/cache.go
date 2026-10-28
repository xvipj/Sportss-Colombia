package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Store abstrae Redis para poder operar en desarrollo sin un servidor Redis.
// La implementación en memoria es un fallback de una sola instancia; en
// producción se debe usar Redis.
type Store interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeletePrefix(ctx context.Context, prefix string) error
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (string, bool, error)
	ReleaseLock(ctx context.Context, key, token string) error
	Ping(ctx context.Context) error
	Name() string
	Close() error
}

// New intenta conectar con Redis. Si no está disponible devuelve un Store en
// memoria y el error para que main pueda advertirlo sin impedir el arranque.
func New(ctx context.Context, addr, password string, db int) (Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return NewMemory(), err
	}
	return &RedisStore{client: client}, nil
}

type RedisStore struct {
	client *redis.Client
}

func (r *RedisStore) Name() string { return "redis" }

func (r *RedisStore) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (r *RedisStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *RedisStore) DeletePrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, prefix+"*", 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

const releaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0`

func (r *RedisStore) AcquireLock(ctx context.Context, key string, ttl time.Duration) (string, bool, error) {
	token := randomToken()
	ok, err := r.client.SetNX(ctx, key, token, ttl).Result()
	return token, ok, err
}

func (r *RedisStore) ReleaseLock(ctx context.Context, key, token string) error {
	return r.client.Eval(ctx, releaseScript, []string{key}, token).Err()
}

func (r *RedisStore) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }
func (r *RedisStore) Close() error                   { return r.client.Close() }

// MemoryStore implementa la misma interfaz con un mutex. Es útil para
// desarrollo local y para que la aplicación siga funcionando sin Redis.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

type memoryItem struct {
	value   string
	expires time.Time
}

func NewMemory() *MemoryStore {
	m := &MemoryStore{items: make(map[string]memoryItem)}
	go m.cleanup()
	return m
}

func (m *MemoryStore) Name() string { return "memoria (Redis no disponible)" }

func (m *MemoryStore) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.RLock()
	it, ok := m.items[key]
	m.mu.RUnlock()
	if !ok {
		return "", false, nil
	}
	if !it.expires.IsZero() && time.Now().After(it.expires) {
		m.mu.Lock()
		delete(m.items, key)
		m.mu.Unlock()
		return "", false, nil
	}
	return it.value, true, nil
}

func (m *MemoryStore) Set(_ context.Context, key, value string, ttl time.Duration) error {
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.items[key] = memoryItem{value: value, expires: exp}
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	for _, k := range keys {
		delete(m.items, k)
	}
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) DeletePrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	for k := range m.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(m.items, k)
		}
	}
	m.mu.Unlock()
	return nil
}

func (m *MemoryStore) AcquireLock(_ context.Context, key string, ttl time.Duration) (string, bool, error) {
	token := randomToken()
	m.mu.Lock()
	defer m.mu.Unlock()
	if it, ok := m.items[key]; ok {
		if it.expires.IsZero() || time.Now().Before(it.expires) {
			return "", false, nil
		}
	}
	m.items[key] = memoryItem{value: token, expires: time.Now().Add(ttl)}
	return token, true, nil
}

func (m *MemoryStore) ReleaseLock(_ context.Context, key, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if it, ok := m.items[key]; ok && it.value == token {
		delete(m.items, key)
	}
	return nil
}

func (m *MemoryStore) Ping(context.Context) error { return nil }
func (m *MemoryStore) Close() error               { return nil }

func (m *MemoryStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for k, it := range m.items {
			if !it.expires.IsZero() && now.After(it.expires) {
				delete(m.items, k)
			}
		}
		m.mu.Unlock()
	}
}

func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}
