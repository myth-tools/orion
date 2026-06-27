package active

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	mrand "math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/myth-tools/orion/internal/dns"
)

var (
	errClientNotInit = errors.New("resolver client not initialized")
	errNoIPs         = errors.New("resolver health check: no IPs returned")
)

type Resolver struct {
	dohResolver *dns.Resolver
	mu          sync.RWMutex
	wildcardIP  map[string]bool
	wildcard    bool
}

func NewResolver(httpClient *http.Client, timeout time.Duration, useDOH bool, resolvers []string) *Resolver {
	if useDOH {
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		return &Resolver{
			dohResolver: dns.NewResolver(httpClient, timeout),
			wildcardIP:  make(map[string]bool),
		}
	}

	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{
		dohResolver: dns.NewUDPResolver(resolvers, timeout),
		wildcardIP:  make(map[string]bool),
	}
}

func (r *Resolver) DetectWildcard(ctx context.Context, domain string) (bool, error) {
	for i := range 3 {
		randBytes := make([]byte, 16)
		if _, err := rand.Read(randBytes); err != nil {
			return false, fmt.Errorf("wildcard rand: %w", err)
		}
		randStr := fmt.Sprintf("wc-test-%x-%d", randBytes, i)
		testDomain := fmt.Sprintf("%s.%s", randStr, domain)

		ips, err := r.LookupSingle(ctx, testDomain)
		if err == nil && len(ips) > 0 {
			r.mu.Lock()
			r.wildcard = true
			for _, ip := range ips {
				r.wildcardIP[ip] = true
			}
			r.mu.Unlock()
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false, nil
}

func (r *Resolver) IsWildcardIP(ip string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.wildcardIP[ip]
}

func (r *Resolver) HasWildcard() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.wildcard
}

func (r *Resolver) LookupSingle(ctx context.Context, domain string) (_ []string, err error) {
	if r.dohResolver == nil {
		return nil, errClientNotInit
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	ips, err := r.dohResolver.LookupIP(ctx, domain)
	if err != nil {
		return nil, err
	}

	if !r.HasWildcard() {
		return ips, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		if !r.wildcardIP[ip] {
			result = append(result, ip)
		}
	}
	return result, nil
}

func (r *Resolver) LookupWithRetry(ctx context.Context, domain string, retries int, timeout time.Duration) ([]string, error) {
	if retries <= 0 {
		retries = 2
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(50*(1<<(attempt-1))) * time.Millisecond
			if attempt > 1 {
				backoff += time.Duration(mrand.Intn(50)) * time.Millisecond
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		subCtx, subCancel := context.WithTimeout(ctx, timeout)
		ips, err := r.LookupSingle(subCtx, domain)
		subCancel()
		if err == nil {
			return ips, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (r *Resolver) Check(ctx context.Context) error {
	if r.dohResolver == nil {
		return errClientNotInit
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ips, err := r.LookupSingle(ctx, "google.com")
	if err != nil {
		return fmt.Errorf("resolver health check failed: %w", err)
	}
	if len(ips) == 0 {
		return errNoIPs
	}
	return nil
}
