package middleware

import (
	"sync"
	"time"
)

type TrafficWindow struct {
	Bytes int64
	Bps   float64
}

type TrafficSnapshot struct {
	W1  TrafficWindow
	W5  TrafficWindow
	W60 TrafficWindow
	H24 TrafficWindow
	H72 TrafficWindow
	D7  TrafficWindow
}

type TrafficStats struct {
	mu      sync.Mutex
	buckets []int64
	marks   []int64
	size    int64
}

func NewTrafficStats() *TrafficStats {
	const minutesInWeek = 7 * 24 * 60
	return &TrafficStats{
		buckets: make([]int64, minutesInWeek),
		marks:   make([]int64, minutesInWeek),
		size:    minutesInWeek,
	}
}

func (t *TrafficStats) Add(bytes int, now time.Time) {
	minute := now.Unix() / 60
	idx := minute % t.size

	t.mu.Lock()
	if t.marks[idx] != minute {
		t.marks[idx] = minute
		t.buckets[idx] = 0
	}
	t.buckets[idx] += int64(bytes)
	t.mu.Unlock()
}

func (t *TrafficStats) Snapshot(now time.Time) TrafficSnapshot {
	current := now.Unix() / 60

	sum := func(minutes int64) int64 {
		var total int64
		for i := int64(0); i < minutes; i++ {
			minute := current - i
			idx := minute % t.size
			if t.marks[idx] == minute {
				total += t.buckets[idx]
			}
		}
		return total
	}

	window := func(minutes int64) TrafficWindow {
		bytes := sum(minutes)
		seconds := float64(minutes * 60)
		var bps float64
		if seconds > 0 {
			bps = float64(bytes) / seconds
		}
		return TrafficWindow{Bytes: bytes, Bps: bps}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return TrafficSnapshot{
		W1:  window(1),
		W5:  window(5),
		W60: window(60),
		H24: window(24 * 60),
		H72: window(72 * 60),
		D7:  window(7 * 24 * 60),
	}
}
