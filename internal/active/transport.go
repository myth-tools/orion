package active

import (
	"context"
	"crypto/tls"
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
	errUnsupportedScheme = errors.New("unsupported proxy scheme (use socks5:// or socks5h://)")
	errCircuitOpen       = errors.New("proxy circuit breaker: open — tunnel unreachable")
)

type ProxyDialer struct {
	inner     proxy.Dialer
	enabled   bool
	mu        sync.RWMutex
	tripped   bool
	tripCount int
}

const maxTripRetries = 3

func NewProxyDialer(proxyURL string) (*ProxyDialer, error) {
	if proxyURL == "" {
		return &ProxyDialer{enabled: false}, nil
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
	}

	var d proxy.Dialer
	switch u.Scheme {
	case "socks5", "socks5h":
		d, err = proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 dialer for %q: %w", proxyURL, err)
		}
	default:
		return nil, fmt.Errorf("%w: got %q", errUnsupportedScheme, u.Scheme)
	}

	return &ProxyDialer{inner: d, enabled: true}, nil
}

func (p *ProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	p.mu.RLock()
	if p.tripped {
		p.mu.RUnlock()
		return nil, errCircuitOpen
	}
	p.mu.RUnlock()

	if !p.enabled {
		d := net.Dialer{Timeout: 5 * time.Second}
		return d.DialContext(ctx, network, addr)
	}

	conn, err := p.inner.Dial(network, addr)
	if err != nil {
		p.mu.Lock()
		p.tripCount++
		tc := p.tripCount
		if tc >= maxTripRetries {
			p.tripped = true
		}
		p.mu.Unlock()
		return nil, fmt.Errorf("proxy connection to %s/%s failed (attempt %d): %w", network, addr, tc, err)
	}

	p.mu.Lock()
	p.tripCount = 0
	p.mu.Unlock()
	return conn, nil
}

func (p *ProxyDialer) Reset() {
	p.mu.Lock()
	p.tripped = false
	p.tripCount = 0
	p.mu.Unlock()
}

func (p *ProxyDialer) Enabled() bool {
	return p.enabled
}

func (p *ProxyDialer) Tripped() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tripped
}

func newProxyTransport(dialer *ProxyDialer) *http.Transport {
	dialContext := dialer.DialContext

	if !dialer.Enabled() {
		d := &net.Dialer{Timeout: 5 * time.Second}
		dialContext = d.DialContext
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	forceHTTP2 := !dialer.Enabled()

	return &http.Transport{
		Proxy:                  nil,
		DialContext:            dialContext,
		MaxIdleConns:           1000,
		MaxConnsPerHost:        500,
		MaxIdleConnsPerHost:    500,
		IdleConnTimeout:        90 * time.Second,
		ResponseHeaderTimeout:  10 * time.Second,
		ExpectContinueTimeout:  1 * time.Second,
		TLSClientConfig:        tlsConfig,
		TLSHandshakeTimeout:    5 * time.Second,
		DisableKeepAlives:      false,
		DisableCompression:     false,
		ForceAttemptHTTP2:      forceHTTP2,
		WriteBufferSize:        65536,
		ReadBufferSize:         65536,
		MaxResponseHeaderBytes: 65536,
	}
}

func NewProxiedHTTPClient(dialer *ProxyDialer, timeout int) *http.Client {
	return &http.Client{
		Transport: newProxyTransport(dialer),
		Timeout:   time.Duration(timeout+10) * time.Second,
	}
}
