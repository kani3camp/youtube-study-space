package mypage

import (
	"math"
	"sync"
	"time"
)

type (
	bucket struct {
		tokens float64
		last   time.Time
	}
	processLimiter struct {
		mu      sync.Mutex
		buckets map[string]bucket
	}
)

func (l *processLimiter) allow(key string, ratePerMinute, burst float64, now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = map[string]bucket{}
	}
	value, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= 4096 {
			for key, value := range l.buckets {
				if now.Sub(value.last) >= time.Hour {
					delete(l.buckets, key)
				}
			}
		}
		if len(l.buckets) >= 4096 {
			return false, 60
		}
		value = bucket{tokens: burst, last: now}
	}
	value.tokens = math.Min(burst, value.tokens+math.Max(0, now.Sub(value.last).Minutes())*ratePerMinute)
	value.last = now
	if value.tokens < 1 {
		l.buckets[key] = value
		return false, int(math.Ceil((1 - value.tokens) * 60 / ratePerMinute))
	}
	value.tokens--
	l.buckets[key] = value
	return true, 0
}
