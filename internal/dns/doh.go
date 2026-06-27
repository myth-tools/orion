package dns

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

var (
	errDNSStatus   = errors.New("non-zero DNS status from DoH provider")
	errNonOKStatus = errors.New("non-200 HTTP status from DoH provider")
	errPackDNSMsg  = errors.New("failed to pack DNS message")
	errCreateReq   = errors.New("failed to create HTTP request")
	errDoHHTTP     = errors.New("DoH HTTP request failed")
	errReadBody    = errors.New("failed to read response body")
	errUnpackDNS   = errors.New("failed to unpack DNS response")
)

type Provider struct {
	Name   string
	URL    string
	Method string
}

var DefaultProviders = []Provider{
	{Name: "cloudflare", URL: "https://cloudflare-dns.com/dns-query", Method: "POST"},
	{Name: "google", URL: "https://dns.google/dns-query", Method: "POST"},
	{Name: "quad9", URL: "https://dns.quad9.net/dns-query", Method: "POST"},
	{Name: "opendns", URL: "https://doh.opendns.com/dns-query", Method: "POST"},
	{Name: "cleanbrowsing", URL: "https://doh.cleanbrowsing.org/doh/dns-query", Method: "POST"},
	{Name: "adguard", URL: "https://dns.adguard-dns.com/dns-query", Method: "POST"},
	{Name: "mullvad", URL: "https://adblock.doh.mullvad.net/dns-query", Method: "POST"},
	{Name: "controld", URL: "https://freedns.controld.com/p1", Method: "POST"},
	{Name: "nextdns", URL: "https://dns.nextdns.io/dns-query", Method: "POST"},
	{Name: "dnsforge", URL: "https://dnsforge.de/dns-query", Method: "POST"},
}

type result struct {
	ips    []string
	status int
}

type Resolver struct {
	providers []Provider
	client    *http.Client
	counter   atomic.Uint64
	timeout   time.Duration
	useDOH    bool
	udpAddrs  []string
}

func NewResolver(client *http.Client, timeout time.Duration) *Resolver {
	providers := make([]Provider, len(DefaultProviders))
	copy(providers, DefaultProviders)
	if client == nil {
		client = http.DefaultClient
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{
		providers: providers,
		client:    client,
		timeout:   timeout,
		useDOH:    true,
	}
}

func NewUDPResolver(addrs []string, timeout time.Duration) *Resolver {
	if len(addrs) == 0 {
		addrs = []string{"1.1.1.1:53", "8.8.8.8:53", "9.9.9.9:53"}
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Resolver{
		useDOH:   false,
		udpAddrs: addrs,
		timeout:  timeout,
	}
}

func (r *Resolver) AddProvider(p Provider) {
	r.providers = append(r.providers, p)
}

func (r *Resolver) SetProviders(providers []Provider) {
	r.providers = providers
}

func (r *Resolver) nextProvider() Provider {
	n := r.counter.Add(1) - 1
	return r.providers[n%uint64(len(r.providers))]
}

func (r *Resolver) Lookup(ctx context.Context, domain string, queryType uint16) ([]string, error) {
	if !r.useDOH {
		return r.lookupUDP(ctx, domain, queryType)
	}

	domain = dns.Fqdn(domain)

	m := new(dns.Msg)
	m.SetQuestion(domain, queryType)
	m.RecursionDesired = true
	m.SetEdns0(1232, false)

	wire, err := m.Pack()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errPackDNSMsg, err)
	}

	perAttempt := r.timeout
	const maxPerAttempt = 3 * time.Second
	if perAttempt > maxPerAttempt {
		perAttempt = maxPerAttempt
	}

	var lastErr error
	for range len(r.providers) {
		provider := r.nextProvider()

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, ctx.Err()
		default:
		}

		attemptCtx, attemptCancel := context.WithTimeout(ctx, perAttempt)

		var res *result
		switch provider.Method {
		case http.MethodGet:
			b64 := base64.RawURLEncoding.EncodeToString(wire)
			res, err = r.doGet(attemptCtx, provider, b64)
		default:
			res, err = r.doPost(attemptCtx, provider, wire)
		}
		attemptCancel()

		if err != nil {
			lastErr = err
			continue
		}
		if res.status == dns.RcodeServerFailure || res.status == dns.RcodeRefused {
			lastErr = fmt.Errorf("%s: %w: %d", provider.Name, errDNSStatus, res.status)
			continue
		}

		return res.ips, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ctx.Err()
}

func (r *Resolver) lookupUDP(ctx context.Context, domain string, queryType uint16) ([]string, error) {
	domain = dns.Fqdn(domain)

	m := new(dns.Msg)
	m.SetQuestion(domain, queryType)
	m.RecursionDesired = true

	client := &dns.Client{Timeout: r.timeout}

	var lastErr error
	for _, addr := range r.udpAddrs {
		resp, _, err := client.ExchangeContext(ctx, m, addr)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.Rcode == dns.RcodeServerFailure || resp.Rcode == dns.RcodeRefused {
			lastErr = fmt.Errorf("UDP DNS %s: %w: %d", addr, errDNSStatus, resp.Rcode)
			continue
		}
		return msgToResult(resp).ips, nil
	}
	return nil, lastErr
}

func (r *Resolver) LookupA(ctx context.Context, domain string) ([]string, error) {
	return r.Lookup(ctx, domain, dns.TypeA)
}

func (r *Resolver) LookupAAAA(ctx context.Context, domain string) ([]string, error) {
	return r.Lookup(ctx, domain, dns.TypeAAAA)
}

func (r *Resolver) LookupCNAME(ctx context.Context, domain string) ([]string, error) {
	return r.Lookup(ctx, domain, dns.TypeCNAME)
}

func (r *Resolver) LookupNS(ctx context.Context, domain string) ([]string, error) {
	return r.Lookup(ctx, domain, dns.TypeNS)
}

func (r *Resolver) LookupIP(ctx context.Context, domain string) ([]string, error) {
	type ipResult struct {
		ips []string
		err error
	}

	aCh := make(chan ipResult, 1)
	aaaaCh := make(chan ipResult, 1)

	go func() {
		ips, err := r.Lookup(ctx, domain, dns.TypeA)
		aCh <- ipResult{ips, err}
	}()
	go func() {
		ips, err := r.Lookup(ctx, domain, dns.TypeAAAA)
		aaaaCh <- ipResult{ips, err}
	}()

	aRes := <-aCh
	aaaaRes := <-aaaaCh

	var all []string
	if aRes.err == nil {
		all = append(all, aRes.ips...)
	}
	if aaaaRes.err == nil {
		all = append(all, aaaaRes.ips...)
	}

	if len(all) > 0 {
		return all, nil
	}
	if aRes.err != nil {
		return nil, aRes.err
	}
	return nil, aaaaRes.err
}

func (r *Resolver) doPost(ctx context.Context, provider Provider, wire []byte) (*result, error) {
	subCtx, subCancel := context.WithTimeout(ctx, r.timeout)
	defer subCancel()

	req, err := http.NewRequestWithContext(subCtx, http.MethodPost, provider.URL, bytes.NewReader(wire))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCreateReq, err)
	}

	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("User-Agent", randomUA())

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", provider.Name, errDoHHTTP, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %w: %d", provider.Name, errNonOKStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", provider.Name, errReadBody, err)
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(body); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", provider.Name, errUnpackDNS, err)
	}

	return msgToResult(msg), nil
}

func (r *Resolver) doGet(ctx context.Context, provider Provider, b64 string) (*result, error) {
	subCtx, subCancel := context.WithTimeout(ctx, r.timeout)
	defer subCancel()

	fullURL := provider.URL + "?dns=" + b64
	req, err := http.NewRequestWithContext(subCtx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCreateReq, err)
	}

	req.Header.Set("Accept", "application/dns-message")
	req.Header.Set("User-Agent", randomUA())

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", provider.Name, errDoHHTTP, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %w: %d", provider.Name, errNonOKStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", provider.Name, errReadBody, err)
	}

	msg := new(dns.Msg)
	if err := msg.Unpack(body); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", provider.Name, errUnpackDNS, err)
	}

	return msgToResult(msg), nil
}

func msgToResult(m *dns.Msg) *result {
	res := &result{status: m.Rcode}
	for _, ans := range m.Answer {
		switch v := ans.(type) {
		case *dns.A:
			res.ips = append(res.ips, v.A.String())
		case *dns.AAAA:
			res.ips = append(res.ips, v.AAAA.String())
		case *dns.CNAME:
			res.ips = append(res.ips, strings.TrimSuffix(v.Target, "."))
		}
	}
	return res
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:122.0) Gecko/20100101 Firefox/122.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
}

func randomUA() string {
	return userAgents[counter.Add(1)%uint64(len(userAgents))]
}

var counter atomic.Uint64
