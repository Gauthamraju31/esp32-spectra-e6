package ratelimit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Limiter tracks daily usage and enforces a rate limit.
type Limiter struct {
	maxPerDay int
	count     int
	resetDay  int // day of year when count was last reset
	filePath  string
	mu        sync.Mutex
}

// limiterState is used for JSON serialization
type limiterState struct {
	Count    int `json:"count"`
	ResetDay int `json:"resetDay"`
}

// NewLimiter creates a new daily rate limiter and loads existing state from disk.
func NewLimiter(maxPerDay int, filePath string) *Limiter {
	l := &Limiter{
		maxPerDay: maxPerDay,
		filePath:  filePath,
		resetDay:  time.Now().UTC().YearDay(),
	}

	if data, err := os.ReadFile(filePath); err == nil {
		var state limiterState
		if err := json.Unmarshal(data, &state); err == nil {
			l.count = state.Count
			l.resetDay = state.ResetDay
		}
	}

	l.mu.Lock()
	l.checkReset()
	l.mu.Unlock()

	return l
}

// save persists the current state to disk. Must be called with lock held.
func (l *Limiter) save() {
	if l.filePath == "" {
		return
	}
	state := limiterState{
		Count:    l.count,
		ResetDay: l.resetDay,
	}
	if data, err := json.Marshal(state); err == nil {
		if err := os.MkdirAll(filepath.Dir(l.filePath), 0755); err == nil {
			_ = os.WriteFile(l.filePath, data, 0644)
		}
	}
}

// checkReset resets the counter if we've moved to a new day.
func (l *Limiter) checkReset() {
	today := time.Now().UTC().YearDay()
	if today != l.resetDay {
		l.count = 0
		l.resetDay = today
		l.save()
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
	l.save()
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
