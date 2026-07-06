package v1

import (
	"sync"
	"time"
)

const defaultMaxEntries = 10000

// loginRateLimiter provides in-memory rate limiting for login endpoints.
// Uses a sliding window counter per IP address with bounded map size.
type loginRateLimiter struct {
	mu         sync.Mutex
	windows    map[string]*loginWindow
	limit      int
	window     time.Duration
	maxEntries int
}

type loginWindow struct {
	count     int
	windowEnd time.Time
}

func newLoginRateLimiter(limit int, window time.Duration) *loginRateLimiter {
	rl := &loginRateLimiter{
		windows:    make(map[string]*loginWindow),
		limit:      limit,
		window:     window,
		maxEntries: defaultMaxEntries,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow returns true if the request is within rate limits.
func (r *loginRateLimiter) Allow(clientIP string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	w, exists := r.windows[clientIP]
	if !exists || now.After(w.windowEnd) {
		// Bounded map: reject new entries if at capacity
		if !exists && len(r.windows) >= r.maxEntries {
			return false
		}
		r.windows[clientIP] = &loginWindow{
			count:     1,
			windowEnd: now.Add(r.window),
		}
		return true
	}

	if w.count >= r.limit {
		return false
	}

	w.count++
	return true
}

func (r *loginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for ip, w := range r.windows {
			if now.After(w.windowEnd) {
				delete(r.windows, ip)
			}
		}
		r.mu.Unlock()
	}
}
