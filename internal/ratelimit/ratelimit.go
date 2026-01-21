package ratelimit

import (
	"sync"
	"time"
)

// Limiter - rate limiter на юзера (sliding window)
type Limiter struct {
	mu       sync.Mutex
	requests map[int64][]time.Time
	limit    int
	window   time.Duration
}

type Config struct {
	RequestsPerMinute int
}

func New(cfg Config) *Limiter {
	limit := cfg.RequestsPerMinute
	if limit <= 0 {
		limit = 10
	}

	l := &Limiter{
		requests: make(map[int64][]time.Time),
		limit:    limit,
		window:   time.Minute,
	}
	go l.cleanup()
	return l
}

func (l *Limiter) Allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// оставляем только свежие запросы
	old := l.requests[userID]
	fresh := old[:0] // reuse underlying array
	for _, t := range old {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}

	if len(fresh) >= l.limit {
		l.requests[userID] = fresh
		return false
	}

	l.requests[userID] = append(fresh, now)
	return true
}

func (l *Limiter) RemainingRequests(userID int64) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.window)
	cnt := 0
	for _, t := range l.requests[userID] {
		if t.After(cutoff) {
			cnt++
		}
	}

	if rem := l.limit - cnt; rem > 0 {
		return rem
	}
	return 0
}

// ResetTime - когда лимит сбросится (приблизительно)
func (l *Limiter) ResetTime(userID int64) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()

	ts := l.requests[userID]
	if len(ts) == 0 {
		return time.Now()
	}

	// ищем самый старый timestamp
	oldest := ts[0]
	for _, t := range ts[1:] {
		if t.Before(oldest) {
			oldest = t
		}
	}
	return oldest.Add(l.window)
}

// cleanup - фоновая очистка старых записей
// TODO: добавить graceful shutdown
func (l *Limiter) cleanup() {
	tick := time.NewTicker(5 * time.Minute)
	for range tick.C {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.window)

		for uid, ts := range l.requests {
			var fresh []time.Time
			for _, t := range ts {
				if t.After(cutoff) {
					fresh = append(fresh, t)
				}
			}
			if len(fresh) == 0 {
				delete(l.requests, uid)
			} else {
				l.requests[uid] = fresh
			}
		}
		l.mu.Unlock()
	}
}
