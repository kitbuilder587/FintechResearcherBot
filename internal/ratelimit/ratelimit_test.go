package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	limiter := New(Config{
		RequestsPerMinute: 3,
	})

	userID := int64(12345)

	for i := 0; i < 3; i++ {
		if !limiter.Allow(userID) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	if limiter.Allow(userID) {
		t.Error("Fourth request should be blocked due to rate limit")
	}
}

func TestLimiter_DifferentUsers(t *testing.T) {
	limiter := New(Config{
		RequestsPerMinute: 1,
	})

	user1 := int64(111)
	user2 := int64(222)

	if !limiter.Allow(user1) {
		t.Error("User1 first request should be allowed")
	}

	if !limiter.Allow(user2) {
		t.Error("User2 first request should be allowed")
	}

	if limiter.Allow(user1) {
		t.Error("User1 second request should be blocked")
	}

	if limiter.Allow(user2) {
		t.Error("User2 second request should be blocked")
	}
}

func TestLimiter_RemainingRequests(t *testing.T) {
	limiter := New(Config{
		RequestsPerMinute: 5,
	})

	userID := int64(12345)

	if remaining := limiter.RemainingRequests(userID); remaining != 5 {
		t.Errorf("RemainingRequests() = %d, want 5", remaining)
	}

	limiter.Allow(userID)
	limiter.Allow(userID)
	limiter.Allow(userID)

	if remaining := limiter.RemainingRequests(userID); remaining != 2 {
		t.Errorf("RemainingRequests() = %d, want 2", remaining)
	}

	limiter.Allow(userID)
	limiter.Allow(userID)

	if remaining := limiter.RemainingRequests(userID); remaining != 0 {
		t.Errorf("RemainingRequests() = %d, want 0", remaining)
	}
}

func TestLimiter_ResetTime(t *testing.T) {
	limiter := New(Config{
		RequestsPerMinute: 1,
	})

	userID := int64(12345)

	before := time.Now()
	limiter.Allow(userID)

	resetTime := limiter.ResetTime(userID)

	expectedReset := before.Add(time.Minute)
	tolerance := 2 * time.Second

	if resetTime.Before(expectedReset.Add(-tolerance)) || resetTime.After(expectedReset.Add(tolerance)) {
		t.Errorf("ResetTime() = %v, expected around %v", resetTime, expectedReset)
	}
}

func TestLimiter_DefaultConfig(t *testing.T) {
	limiter := New(Config{
		RequestsPerMinute: 0,
	})

	userID := int64(12345)

	for i := 0; i < 10; i++ {
		if !limiter.Allow(userID) {
			t.Errorf("Request %d should be allowed with default config", i+1)
		}
	}

	// 11th should be blocked
	if limiter.Allow(userID) {
		t.Error("11th request should be blocked")
	}
}

func TestLimiter_Concurrent(t *testing.T) {
	limiter := New(Config{
		RequestsPerMinute: 100,
	})

	done := make(chan bool)
	userID := int64(12345)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				limiter.Allow(userID)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	remaining := limiter.RemainingRequests(userID)
	if remaining != 0 {
		t.Errorf("RemainingRequests() = %d, want 0 after concurrent access", remaining)
	}
}
