package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

var (
	errUnsupportedProxyType = errors.New("unsupported proxy type")
	errSOCKS4Unsupported    = errors.New("SOCKS4 unsupported by Go standard library; use SOCKS5")
	errNoSOCKS5InPool       = errors.New("no working SOCKS5 proxy available in pool")
)

const maxProxyRetries = 3

type RotatingTransport struct {
	pool   *Pool
	base   *http.Transport
	mu     sync.Mutex
	failed map[string]int
}

func NewRotatingTransport(pool *Pool) *RotatingTransport {
	return &RotatingTransport{
		pool:   pool,
		failed: make(map[string]int),
		base: &http.Transport{
			MaxIdleConns:        50,
			MaxConnsPerHost:     10,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false,
			TLSHandshakeTimeout: 5 * time.Second,
			ForceAttemptHTTP2:   false,
		},
	}
}

// socksDialContext wraps a golang.org/x/net/proxy.Dialer (which lacks
// context support) so that context cancellation aborts an in-flight SOCKS5
// dial. Without this, an HTTP request can hang until the SOCKS5 handshake
// completes even when the caller's context has expired.
func socksDialContext(d proxy.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		type connResult struct {
			conn net.Conn
			err  error
		}
		ch := make(chan connResult, 1)
		go func() {
			conn, err := d.Dial(network, addr)
			ch <- connResult{conn, err}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-ch:
			return r.conn, r.err
		}
	}
}

func (t *RotatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req, 0)
}

func (t *RotatingTransport) roundTrip(req *http.Request, retries int) (*http.Response, error) {
	if retries >= maxProxyRetries {
		return t.base.RoundTrip(req)
	}

	pr, ok := t.pool.GetNext()
	if !ok {
		return t.base.RoundTrip(req)
	}

	transport, err := transportForProxy(pr, t.base)
	if err != nil {
		t.pool.Remove(pr.Addr)
		return t.roundTrip(req, retries+1)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.mu.Lock()
		t.failed[pr.Addr]++
		fails := t.failed[pr.Addr]
		t.mu.Unlock()
		if fails >= 2 {
			t.pool.Remove(pr.Addr)
		}
		return t.roundTrip(req, retries+1)
	}

	return resp, nil
}

func transportForProxy(pr Proxy, base *http.Transport) (*http.Transport, error) {
	clone := base.Clone()
	clone.DisableKeepAlives = true

	switch pr.Type {
	case ProxyTypeHTTP, "https":
		proxyURL, err := url.Parse("http://" + pr.Addr)
		if err != nil {
			return nil, err
		}
		clone.Proxy = http.ProxyURL(proxyURL)
	case ProxyTypeSOCKS5:
		proxyURL := &url.URL{Scheme: "socks5", Host: pr.Addr}
		if pr.Username != "" {
			proxyURL.User = url.UserPassword(pr.Username, pr.Password)
		}
		socksDialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, err
		}
		clone.DialContext = socksDialContext(socksDialer)
	case ProxyTypeSOCKS4:
		return nil, fmt.Errorf("%w: %s: %s", errSOCKS4Unsupported, pr.Addr, pr.Type)
	default:
		return nil, fmt.Errorf("%w: %s: %s", errUnsupportedProxyType, pr.Addr, pr.Type)
	}

	return clone, nil
}

func (t *RotatingTransport) NewHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &http.Client{
		Transport: t,
		Timeout:   timeout,
	}
}

func (t *RotatingTransport) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	for range 20 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pr, ok := t.pool.GetNext()
		if !ok {
			break
		}
		if pr.Type != ProxyTypeSOCKS5 {
			continue
		}

		proxyURL := &url.URL{Scheme: "socks5", Host: pr.Addr}
		if pr.Username != "" {
			proxyURL.User = url.UserPassword(pr.Username, pr.Password)
		}
		d, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			t.pool.Remove(pr.Addr)
			continue
		}
		conn, err := d.Dial(network, addr)
		if err != nil {
			t.pool.Remove(pr.Addr)
			continue
		}
		return conn, nil
	}
	return nil, fmt.Errorf("%w: tried %d SOCKS5 candidates", errNoSOCKS5InPool, 20)
}
