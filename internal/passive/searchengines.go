package passive

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type Bing struct{}

func (b *Bing) Name() string       { return "bing" }
func (b *Bing) NeedsKey() bool     { return false }
func (b *Bing) SetKeys(_ []string) {}

func (b *Bing) Fetch(ctx context.Context, domain string, results chan<- string) error {
	apiURL := fmt.Sprintf("https://www.bing.com/search?q=site:*.%s&count=50", domain)
	body, err := fetch(ctx, apiURL)
	if err != nil {
		return fmt.Errorf("bing: %w", err)
	}
	extractSearchResults(ctx, body, domain, results)
	return nil
}

type Baidu struct{}

func (b *Baidu) Name() string       { return "baidu" }
func (b *Baidu) NeedsKey() bool     { return false }
func (b *Baidu) SetKeys(_ []string) {}

func (b *Baidu) Fetch(ctx context.Context, domain string, results chan<- string) error {
	apiURL := fmt.Sprintf("https://www.baidu.com/s?wd=site:%s&rn=50", domain)
	body, err := fetch(ctx, apiURL)
	if err != nil {
		return fmt.Errorf("baidu: %w", err)
	}
	extractSearchResults(ctx, body, domain, results)
	return nil
}

var hrefRegex = regexp.MustCompile(`<a[^>]+href="(https?://[^"]+)"`)

func extractSearchResults(ctx context.Context, html []byte, domain string, results chan<- string) {
	seen := make(map[string]bool)
	matches := hrefRegex.FindAllSubmatch(html, -1)
	for _, m := range matches {
		rawURL := string(m[1])
		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		hostname := strings.ToLower(parsed.Hostname())
		if hostname == "" || !strings.HasSuffix(hostname, "."+domain) || hostname == domain {
			continue
		}
		if seen[hostname] {
			continue
		}
		seen[hostname] = true
		select {
		case results <- hostname:
		case <-ctx.Done():
			return
		}
	}
}
