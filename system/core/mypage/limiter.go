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
		starts  map[string][]time.Time
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

func (l *processLimiter) allowStart(ip string, now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.starts == nil {
		l.starts = map[string][]time.Time{}
	}
	if _, ok := l.starts[ip]; !ok && len(l.starts) >= 4096 {
		for key, values := range l.starts {
			if len(values) == 0 || now.Sub(values[len(values)-1]) >= time.Hour {
				delete(l.starts, key)
			}
		}
		if len(l.starts) >= 4096 {
			return false, 600
		}
	}
	recent := make([]time.Time, 0, 20)
	short := 0
	var oldestShort time.Time
	for _, at := range l.starts[ip] {
		if now.Sub(at) < time.Hour {
			recent = append(recent, at)
			if now.Sub(at) < 10*time.Minute {
				short++
				if oldestShort.IsZero() {
					oldestShort = at
				}
			}
		}
	}
	l.starts[ip] = recent
	if short >= 5 {
		return false, int(math.Ceil(oldestShort.Add(10 * time.Minute).Sub(now).Seconds()))
	}
	if len(recent) >= 20 {
		return false, int(math.Ceil(recent[0].Add(time.Hour).Sub(now).Seconds()))
	}
	l.starts[ip] = append(recent, now)
	return true, 0
}
