package proxy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var errNoWorkingProxies = errors.New("no working proxies found after scrape")

type Pool struct {
	mu       sync.RWMutex
	proxies  []Proxy
	custom   []Proxy
	counter  atomic.Uint64
	scraper  *Scraper
	tester   *Tester
	interval time.Duration
	stopCh   chan struct{}
	stopped  atomic.Bool
}

func NewPool(scraper *Scraper, tester *Tester, refreshInterval time.Duration) *Pool {
	if refreshInterval <= 0 {
		refreshInterval = 10 * time.Minute
	}
	return &Pool{
		scraper:  scraper,
		tester:   tester,
		interval: refreshInterval,
		stopCh:   make(chan struct{}),
	}
}

func (p *Pool) AddURL(rawURL string) error {
	pr, err := ParseProxyURL(rawURL)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, existing := range p.proxies {
		if existing.Addr == pr.Addr && existing.Type == pr.Type {
			return nil
		}
	}
	p.custom = append(p.custom, pr)
	p.proxies = append(p.proxies, pr)
	return nil
}

func (p *Pool) Start(ctx context.Context) error {
	startCtx, startCancel := context.WithTimeout(ctx, 45*time.Second)
	defer startCancel()

	proxies, err := p.scraper.Scrape(startCtx)
	if err != nil {
		return fmt.Errorf("initial scrape: %w", err)
	}

	valid := p.tester.Test(startCtx, proxies)
	if len(valid) == 0 {
		return fmt.Errorf("%w: %d candidates tested", errNoWorkingProxies, len(proxies))
	}

	p.mu.Lock()
	p.proxies = valid
	p.proxies = append(p.proxies, p.custom...)
	p.mu.Unlock()

	go p.refreshLoop(ctx)
	return nil
}

func (p *Pool) Stop() {
	if p.stopped.CompareAndSwap(false, true) {
		close(p.stopCh)
	}
}

func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}

func (p *Pool) GetNext() (Proxy, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	n := len(p.proxies)
	if n == 0 {
		return Proxy{}, false
	}

	idx := p.counter.Add(1) - 1
	return p.proxies[idx%uint64(n)], true
}

func (p *Pool) Remove(addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, pr := range p.proxies {
		if pr.Addr == addr {
			p.proxies = append(p.proxies[:i], p.proxies[i+1:]...)
			break
		}
	}
	for i, pr := range p.custom {
		if pr.Addr == addr {
			p.custom = append(p.custom[:i], p.custom[i+1:]...)
			break
		}
	}
}

func (p *Pool) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.refresh(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pool) refresh(ctx context.Context) {
	refreshCtx, refreshCancel := context.WithTimeout(ctx, 30*time.Second)
	defer refreshCancel()

	candidates, err := p.scraper.Scrape(refreshCtx)
	if err != nil {
		return
	}

	valid := p.tester.Test(refreshCtx, candidates)
	if len(valid) == 0 {
		return
	}

	p.mu.Lock()
	seen := make(map[string]bool, len(valid)+len(p.custom))
	p.proxies = make([]Proxy, 0, len(valid)+len(p.custom))
	for _, pr := range valid {
		key := pr.Type + "://" + pr.Addr
		if !seen[key] {
			seen[key] = true
			p.proxies = append(p.proxies, pr)
		}
	}
	for _, pr := range p.custom {
		key := pr.Type + "://" + pr.Addr
		if !seen[key] {
			seen[key] = true
			p.proxies = append(p.proxies, pr)
		}
	}
	p.mu.Unlock()
}
