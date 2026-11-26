package orderbook_service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"metaid-market-service/conf"
)

var (
	ErrOutpointLocked = errors.New("order UTXO is currently soft-locked by another pending fill")
)

// SoftLocker prevents two concurrent Takers from selecting the exact same on-chain Order UTXO.
type SoftLocker interface {
	Acquire(outpoints []string, reservationID string, ttl time.Duration) error
	Extend(outpoints []string, reservationID string, ttl time.Duration) error
	Release(outpoints []string, reservationID string) error
	IsLocked(outpoint string) bool
}

// MemorySoftLocker is an in-memory TTL implementation used for unit tests and
// automatic fallback when Redis is disabled or unreachable.
type MemorySoftLocker struct {
	mu    sync.Mutex
	locks map[string]memLockEntry
}

type memLockEntry struct {
	reservationID string
	expiresAt     time.Time
}

func NewMemorySoftLocker() *MemorySoftLocker {
	return &MemorySoftLocker{
		locks: make(map[string]memLockEntry),
	}
}

func (m *MemorySoftLocker) pruneLocked(now time.Time) {
	for k, v := range m.locks {
		if !v.expiresAt.After(now) {
			delete(m.locks, k)
		}
	}
}

func (m *MemorySoftLocker) Acquire(outpoints []string, reservationID string, ttl time.Duration) error {
	if len(outpoints) == 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = 25 * time.Second
	}
	now := time.Now()
	exp := now.Add(ttl)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(now)

	for _, op := range outpoints {
		if existing, ok := m.locks[op]; ok && existing.reservationID != reservationID {
			return fmt.Errorf("%w: %s", ErrOutpointLocked, op)
		}
	}
	for _, op := range outpoints {
		m.locks[op] = memLockEntry{reservationID: reservationID, expiresAt: exp}
	}
	return nil
}

func (m *MemorySoftLocker) Extend(outpoints []string, reservationID string, ttl time.Duration) error {
	if len(outpoints) == 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = 25 * time.Second
	}
	now := time.Now()
	exp := now.Add(ttl)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked(now)
	for _, op := range outpoints {
		if existing, ok := m.locks[op]; !ok || existing.reservationID == reservationID {
			m.locks[op] = memLockEntry{reservationID: reservationID, expiresAt: exp}
		}
	}
	return nil
}

func (m *MemorySoftLocker) Release(outpoints []string, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, op := range outpoints {
		if existing, ok := m.locks[op]; ok && (reservationID == "" || existing.reservationID == reservationID) {
			delete(m.locks, op)
		}
	}
	return nil
}

func (m *MemorySoftLocker) IsLocked(outpoint string) bool {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.locks[outpoint]; ok {
		if existing.expiresAt.After(now) {
			return true
		}
		delete(m.locks, outpoint)
	}
	return false
}

// RedisSoftLocker implements atomic SET ob:lock:<txid>:<vout> <reservationId> NX EX <ttl>
// with safe owner-checked Lua release/extend.
type RedisSoftLocker struct {
	pool     *redigo.Pool
	fallback *MemorySoftLocker
}

var releaseLuaScript = redigo.NewScript(1, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

var extendLuaScript = redigo.NewScript(1, `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

func NewRedisSoftLockerFromConf() SoftLocker {
	if conf.RedisEndpoint == "" {
		return NewMemorySoftLocker()
	}
	endpoint := conf.RedisEndpoint
	password := conf.RedisPassword
	db := conf.RedisDbUtxo
	pool := &redigo.Pool{
		MaxIdle:     16,
		MaxActive:   128,
		IdleTimeout: 120 * time.Second,
		Dial: func() (redigo.Conn, error) {
			opts := []redigo.DialOption{
				redigo.DialConnectTimeout(2 * time.Second),
				redigo.DialReadTimeout(2 * time.Second),
				redigo.DialWriteTimeout(2 * time.Second),
			}
			if password != "" {
				opts = append(opts, redigo.DialPassword(password))
			}
			c, err := redigo.Dial("tcp", endpoint, opts...)
			if err != nil {
				return nil, err
			}
			if db > 0 {
				if _, err := c.Do("SELECT", db); err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			return c, nil
		},
	}
	return &RedisSoftLocker{
		pool:     pool,
		fallback: NewMemorySoftLocker(),
	}
}

func obLockKey(outpoint string) string {
	return "ob:lock:" + outpoint
}

func (r *RedisSoftLocker) Acquire(outpoints []string, reservationID string, ttl time.Duration) error {
	if len(outpoints) == 0 {
		return nil
	}
	ttlSec := int(ttl.Seconds())
	if ttlSec <= 0 {
		ttlSec = 25
	}
	conn := r.pool.Get()
	if conn.Err() != nil {
		_ = conn.Close()
		return r.fallback.Acquire(outpoints, reservationID, ttl)
	}
	defer conn.Close()

	acquired := make([]string, 0, len(outpoints))
	for _, op := range outpoints {
		reply, err := conn.Do("SET", obLockKey(op), reservationID, "EX", ttlSec, "NX")
		if err != nil {
			for _, prev := range acquired {
				_, _ = releaseLuaScript.Do(conn, obLockKey(prev), reservationID)
			}
			return r.fallback.Acquire(outpoints, reservationID, ttl)
		}
		if reply == nil {
			// Check if already owned by this reservationID
			cur, _ := redigo.String(conn.Do("GET", obLockKey(op)))
			if cur != reservationID {
				for _, prev := range acquired {
					_, _ = releaseLuaScript.Do(conn, obLockKey(prev), reservationID)
				}
				return fmt.Errorf("%w: %s", ErrOutpointLocked, op)
			}
		}
		acquired = append(acquired, op)
	}
	return nil
}

func (r *RedisSoftLocker) Extend(outpoints []string, reservationID string, ttl time.Duration) error {
	if len(outpoints) == 0 {
		return nil
	}
	ttlSec := int(ttl.Seconds())
	if ttlSec <= 0 {
		ttlSec = 25
	}
	conn := r.pool.Get()
	if conn.Err() != nil {
		_ = conn.Close()
		return r.fallback.Extend(outpoints, reservationID, ttl)
	}
	defer conn.Close()
	for _, op := range outpoints {
		_, _ = extendLuaScript.Do(conn, obLockKey(op), reservationID, ttlSec)
	}
	return nil
}

func (r *RedisSoftLocker) Release(outpoints []string, reservationID string) error {
	if len(outpoints) == 0 {
		return nil
	}
	conn := r.pool.Get()
	if conn.Err() != nil {
		_ = conn.Close()
		return r.fallback.Release(outpoints, reservationID)
	}
	defer conn.Close()
	for _, op := range outpoints {
		_, _ = releaseLuaScript.Do(conn, obLockKey(op), reservationID)
	}
	return nil
}

func (r *RedisSoftLocker) IsLocked(outpoint string) bool {
	conn := r.pool.Get()
	if conn.Err() != nil {
		_ = conn.Close()
		return r.fallback.IsLocked(outpoint)
	}
	defer conn.Close()
	exists, err := redigo.Int(conn.Do("EXISTS", obLockKey(outpoint)))
	if err != nil {
		return r.fallback.IsLocked(outpoint)
	}
	return exists > 0
}
