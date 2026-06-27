package proxy

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var testURLs = []string{
	"http://www.gstatic.com/generate_204",
}

type Tester struct {
	clientTimeout time.Duration
	testURLs      []string
	concurrency   int
}

func NewTester(timeout time.Duration, concurrency int) *Tester {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if concurrency <= 0 {
		concurrency = 200
	}
	urls := make([]string, len(testURLs))
	copy(urls, testURLs)
	return &Tester{
		clientTimeout: timeout,
		testURLs:      urls,
		concurrency:   concurrency,
	}
}

func (t *Tester) SetTestURL(url string) {
	t.testURLs = []string{url}
}

func (t *Tester) Test(ctx context.Context, proxies []Proxy) []Proxy {
	if len(proxies) == 0 {
		return nil
	}

	testCtx, testCancel := context.WithCancel(ctx)
	defer testCancel()

	sem := make(chan struct{}, t.concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var alive []Proxy

	for _, p := range proxies {
		select {
		case <-testCtx.Done():
			testCancel()
			wg.Wait()
			return alive
		default:
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(pr Proxy) {
			defer wg.Done()
			defer func() { <-sem }()

			if t.testProxy(testCtx, pr) {
				mu.Lock()
				pr.Alive = true
				alive = append(alive, pr)
				mu.Unlock()
			}
		}(p)
	}

	testCancel()
	wg.Wait()
	return alive
}

func (t *Tester) testProxy(ctx context.Context, p Proxy) bool {
	testCtx, testCancel := context.WithTimeout(ctx, t.clientTimeout)
	defer testCancel()

	for _, url := range t.testURLs {
		ok := t.testSingleURL(testCtx, p, url)
		if ok {
			return true
		}
	}
	return false
}

func (t *Tester) testSingleURL(ctx context.Context, p Proxy, testURL string) bool {
	switch p.Type {
	case ProxyTypeSOCKS5, ProxyTypeSOCKS4:
		return t.testSOCKS(ctx, p, testURL)
	default:
		return t.testHTTP(ctx, p, testURL)
	}
}

func (t *Tester) testHTTP(ctx context.Context, p Proxy, testURL string) bool {
	proxyURL, err := url.Parse("http://" + p.Addr)
	if err != nil {
		return false
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   t.clientTimeout,
			KeepAlive: 0,
		}).DialContext,
		TLSHandshakeTimeout: t.clientTimeout,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   t.clientTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, testURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()

	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent
}

func (t *Tester) testSOCKS(ctx context.Context, p Proxy, testURL string) bool {
	proxyURL := &url.URL{Scheme: "socks5", Host: p.Addr}
	if p.Username != "" {
		proxyURL.User = url.UserPassword(p.Username, p.Password)
	}

	socksDialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		return false
	}

	transport := &http.Transport{
		DialContext:         socksDialContext(socksDialer),
		TLSHandshakeTimeout: t.clientTimeout,
		DisableKeepAlives:   true,
		ForceAttemptHTTP2:   false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   t.clientTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, testURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()

	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent
}
