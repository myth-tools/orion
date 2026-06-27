package active

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/myth-tools/orion/internal/types"
)

const (
	defaultDNSTimeout   = 5
	defaultDNSRate      = 1000
	defaultDNSRetries   = 2
	dnsTimeoutSlack     = 2
	stopErrorThreshold  = 0.5
	stopErrorSampleSize = 100
	stopErrorMinSamples = 20
	stopSpuriousMax     = 3
	stopSpuriousWindow  = 500
)

type Bruteforcer struct {
	wordlist    []string
	resolver    *Resolver
	rateTicker  *time.Ticker
	dnsTimeout  time.Duration
	maxRetries  int
	stat        *types.DNSStat
	statMu      sync.Mutex
	errorRing   []bool
	errorRingMu sync.Mutex
	spuriousErr int
}

func NewBruteforcer(wordlist []string, resolver *Resolver, ratePerSec int, timeout int, retries int) *Bruteforcer {
	if ratePerSec <= 0 {
		ratePerSec = defaultDNSRate
	}
	if timeout <= 0 {
		timeout = defaultDNSTimeout
	}
	if retries < 0 {
		retries = defaultDNSRetries
	}
	ticker := rateTicker(ratePerSec)
	return &Bruteforcer{
		wordlist:   wordlist,
		resolver:   resolver,
		rateTicker: ticker,
		dnsTimeout: time.Duration(timeout) * time.Second,
		maxRetries: retries,
		stat:       &types.DNSStat{StartedAt: time.Now()},
		errorRing:  make([]bool, stopErrorSampleSize),
	}
}

func rateTicker(ratePerSec int) *time.Ticker {
	switch {
	case ratePerSec <= 0:
		return time.NewTicker(time.Microsecond)
	case ratePerSec >= 1000000:
		return time.NewTicker(time.Microsecond)
	default:
		interval := max(time.Duration(1_000_000/ratePerSec)*time.Microsecond, time.Microsecond)
		return time.NewTicker(interval)
	}
}

func NewBruteforcerFromFile(path string, resolver *Resolver, ratePerSec int, timeout int, retries int) (*Bruteforcer, error) {
	words, err := loadWordlist(path)
	if err != nil {
		return nil, err
	}
	return NewBruteforcer(words, resolver, ratePerSec, timeout, retries), nil
}

func (b *Bruteforcer) WordlistSize() int {
	return len(b.wordlist)
}

func (b *Bruteforcer) Stat() types.DNSStat {
	b.statMu.Lock()
	defer b.statMu.Unlock()
	return *b.stat
}

func (b *Bruteforcer) Stop() {
	b.rateTicker.Stop()
}

func (b *Bruteforcer) GenerateCandidates(domain string) []string {
	seen := make(map[string]bool)
	var candidates []string
	for _, word := range b.wordlist {
		word = strings.TrimSpace(word)
		if word == "" || strings.HasPrefix(word, "#") {
			continue
		}
		candidate := fmt.Sprintf("%s.%s", word, domain)
		if !seen[candidate] {
			seen[candidate] = true
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

type Config struct {
	Domain  string
	Threads int
	Results chan<- string
	LogFn   func(format string, args ...any)
}

func (b *Bruteforcer) Run(ctx context.Context, cfg Config) {
	candidates := b.GenerateCandidates(cfg.Domain)
	if len(candidates) == 0 {
		return
	}

	b.statMu.Lock()
	b.stat.Total = len(candidates)
	b.statMu.Unlock()

	sem := make(chan struct{}, cfg.Threads)
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer b.rateTicker.Stop()

	lastLog := time.Now()
	lastCheckTime := time.Now()

	for i, candidate := range candidates {
		if stop := b.checkStopConditions(cfg, i); stop {
			break
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		<-b.rateTicker.C
		sem <- struct{}{}
		wg.Add(1)

		go func(c string) {
			defer wg.Done()
			defer func() { <-sem }()

			subCtx, subCancel := context.WithTimeout(ctx, b.dnsTimeout+dnsTimeoutSlack*time.Second)
			defer subCancel()

			ips, err := b.resolveWithRetry(subCtx, c)
			b.recordAttempt(err)

			if err != nil || len(ips) == 0 {
				return
			}

			b.statMu.Lock()
			b.stat.Found++
			b.statMu.Unlock()

			select {
			case cfg.Results <- c:
			case <-ctx.Done():
			}
		}(candidate)

		if time.Since(lastLog) > 2*time.Second {
			lastLog = time.Now()
			b.statMu.Lock()
			pct := 0
			completed := b.stat.Completed
			total := b.stat.Total
			found := b.stat.Found
			errs := b.stat.Errors
			if total > 0 {
				pct = completed * 100 / total
			}
			b.statMu.Unlock()
			if cfg.LogFn != nil {
				cfg.LogFn("[~] Brute-force: %d/%d (%d%%) — %d found, %d err\n",
					completed, total, pct, found, errs)
			}
		}

		_ = lastCheckTime
	}
	wg.Wait()
}

func (b *Bruteforcer) checkStopConditions(cfg Config, idx int) (stop bool) {
	b.statMu.Lock()
	completed := b.stat.Completed
	spurious := b.spuriousErr
	timeoutRate := float64(0)
	if completed > 0 {
		timeoutRate = float64(b.stat.Timeouts) / float64(completed)
	}
	b.statMu.Unlock()

	if idx > stopSpuriousWindow && spurious >= stopSpuriousMax {
		if cfg.LogFn != nil {
			cfg.LogFn("[!] Brute-force: spurious errors > %d — aborting\n", stopSpuriousMax)
		}
		return true
	}

	if completed > stopErrorMinSamples && timeoutRate > 0.5 {
		if cfg.LogFn != nil {
			cfg.LogFn("[!] Brute-force: timeout rate %.0f%% — resolver may be overloaded\n", timeoutRate*100)
		}
		return true
	}

	return shouldStop(b, cfg)
}

func (b *Bruteforcer) resolveWithRetry(ctx context.Context, domain string) ([]string, error) {
	var lastErr error
	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		if attempt > 0 {
			b.statMu.Lock()
			b.stat.Retries++
			b.statMu.Unlock()
			backoff := time.Duration(50*(1<<(attempt-1))) * time.Millisecond
			if attempt > 1 {
				jitter := time.Duration(rand.Intn(50)) * time.Millisecond
				backoff += jitter
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		ips, err := b.resolver.LookupSingle(ctx, domain)
		if err == nil {
			return ips, nil
		}
		lastErr = err
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (b *Bruteforcer) recordAttempt(err error) {
	b.statMu.Lock()
	defer b.statMu.Unlock()

	b.stat.Completed++

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			b.stat.Timeouts++
		} else if errors.Is(err, context.Canceled) {
			return
		}
		b.stat.Errors++
		b.spuriousErr++
	} else {
		b.spuriousErr = 0
	}

	b.errorRingMu.Lock()
	b.errorRing[b.stat.Completed%stopErrorSampleSize] = err != nil
	b.errorRingMu.Unlock()
}

func shouldStop(b *Bruteforcer, cfg Config) bool {
	b.statMu.Lock()
	completed := b.stat.Completed
	b.statMu.Unlock()

	if completed < stopErrorMinSamples {
		return false
	}

	b.errorRingMu.Lock()

	var errCount int
	var total int
	for _, failed := range b.errorRing {
		total++
		if failed {
			errCount++
		}
	}
	b.errorRingMu.Unlock()

	if total == 0 {
		return false
	}
	rate := float64(errCount) / float64(total)
	if rate >= stopErrorThreshold {
		if cfg.LogFn != nil {
			cfg.LogFn("[!] Brute-force: error rate %.0f%% in last %d samples — stopping\n",
				rate*100, total)
		}
		return true
	}
	return false
}

func loadWordlist(path string) ([]string, error) {
	cleanPath := filepath.Clean(path)
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open wordlist: %w", err)
	}
	defer f.Close()

	words := make([]string, 0, 10000)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		words = append(words, scanner.Text())
	}
	return words, scanner.Err()
}
