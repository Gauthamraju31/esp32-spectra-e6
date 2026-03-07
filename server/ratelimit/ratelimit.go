package ratelimit

import (
	"sync"
	"time"
)

// Limiter tracks daily usage and enforces a rate limit.
type Limiter struct {
	maxPerDay int
	count     int
	resetDay  int // day of year when count was last reset
	mu        sync.Mutex
}

// NewLimiter creates a new daily rate limiter.
func NewLimiter(maxPerDay int) *Limiter {
	return &Limiter{
		maxPerDay: maxPerDay,
		resetDay:  time.Now().UTC().YearDay(),
	}
}

// checkReset resets the counter if we've moved to a new day.
func (l *Limiter) checkReset() {
	today := time.Now().UTC().YearDay()
	if today != l.resetDay {
		l.count = 0
		l.resetDay = today
	}
}

// Allow checks if a request is allowed and increments the counter if so.
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.checkReset()

	if l.count >= l.maxPerDay {
		return false
	}
	l.count++
	return true
}

// Remaining returns the number of requests remaining today.
func (l *Limiter) Remaining() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.checkReset()

	remaining := l.maxPerDay - l.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Limit returns the maximum number of requests per day.
func (l *Limiter) Limit() int {
	return l.maxPerDay
}

// Used returns the number of requests used today.
func (l *Limiter) Used() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.checkReset()
	return l.count
}
