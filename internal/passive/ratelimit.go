package passive

import (
	"math"
	"strings"
	"sync"
	"time"
)

type RateLimitEntry struct {
	MaxCount uint
	Duration time.Duration
}

type Limiter struct {
	tokens chan struct{}
	ticker *time.Ticker
	done   chan struct{}
	once   sync.Once
}

func newLimiter(maxCount uint, duration time.Duration) *Limiter {
	if maxCount == 0 || maxCount >= math.MaxUint32 {
		return nil
	}
	l := &Limiter{
		tokens: make(chan struct{}, maxCount),
		ticker: time.NewTicker(duration),
		done:   make(chan struct{}),
	}
	for range maxCount {
		l.tokens <- struct{}{}
	}
	go l.run()
	return l
}

func (l *Limiter) run() {
	for {
		select {
		case <-l.ticker.C:
			l.refill()
		case <-l.done:
			l.ticker.Stop()
			return
		}
	}
}

func (l *Limiter) refill() {
	for {
		select {
		case l.tokens <- struct{}{}:
		default:
			return
		}
	}
}

func (l *Limiter) Take() {
	<-l.tokens
}

func (l *Limiter) Stop() {
	l.once.Do(func() {
		close(l.done)
	})
}

type MultiLimiter struct {
	limiters sync.Map
}

func NewMultiLimiter() *MultiLimiter {
	return &MultiLimiter{}
}

func (m *MultiLimiter) Add(key string, maxCount uint, duration time.Duration) {
	if limiter := newLimiter(maxCount, duration); limiter != nil {
		m.limiters.Store(strings.ToLower(key), limiter)
	}
}

func (m *MultiLimiter) Take(key string) {
	if key == "" {
		return
	}
	if v, ok := m.limiters.Load(strings.ToLower(key)); ok {
		if limiter, ok := v.(*Limiter); ok {
			limiter.Take()
		}
	}
}

func (m *MultiLimiter) Stop() {
	m.limiters.Range(func(_, value any) bool {
		if limiter, ok := value.(*Limiter); ok {
			limiter.Stop()
		}
		return true
	})
}

var DefaultRateLimits = map[string]RateLimitEntry{
	"github":       {MaxCount: 83, Duration: time.Minute},
	"fullhunt":     {MaxCount: 60, Duration: time.Minute},
	"virustotal":   {MaxCount: 4, Duration: time.Minute},
	"hackertarget": {MaxCount: 50, Duration: time.Hour * 24},
	"whoisxmlapi":  {MaxCount: 50, Duration: time.Second},

	"netlas":      {MaxCount: 1, Duration: time.Second},
	"urlscan":     {MaxCount: 1, Duration: time.Second},
	"hudsonrock":  {MaxCount: 5, Duration: time.Second},
	"robtex":      {MaxCount: 10, Duration: time.Hour},
	"wayback":     {MaxCount: 15, Duration: time.Minute},
	"bufferover":  {MaxCount: 60, Duration: time.Minute},
	"leakix":      {MaxCount: 10, Duration: time.Minute},
	"intelx":      {MaxCount: 3, Duration: time.Second},
	"censys":      {MaxCount: 10, Duration: time.Minute},
	"bevigil":     {MaxCount: 60, Duration: time.Minute},
	"builtwith":   {MaxCount: 100, Duration: time.Minute},
	"chaos":       {MaxCount: 10, Duration: time.Minute},
	"dnsdumpster": {MaxCount: 5, Duration: time.Minute},
	"merklemap":   {MaxCount: 10, Duration: time.Minute},
	"zoomeyeapi":  {MaxCount: 10, Duration: time.Minute},
	"digitalyama": {MaxCount: 10, Duration: time.Minute},
	"threatbook":  {MaxCount: 10, Duration: time.Minute},
	"windvane":    {MaxCount: 10, Duration: time.Minute},
	"rsecloud":    {MaxCount: 60, Duration: time.Minute},
}
