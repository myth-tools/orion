package passive

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var googleHrefRegex = regexp.MustCompile(`<a[^>]+href="(https?://[^"]+)"`)

type Google struct{}

func (g *Google) Name() string       { return "google" }
func (g *Google) NeedsKey() bool     { return false }
func (g *Google) SetKeys(_ []string) {}

func (g *Google) Fetch(ctx context.Context, domain string, results chan<- string) error {
	seen := make(map[string]bool)

	start := 0
	for range 5 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		encodedDomain := url.QueryEscape(domain)
		searchURL := fmt.Sprintf("https://www.google.com/search?q=site:*.%s&start=%d&num=100", encodedDomain, start)
		body, err := fetch(ctx, searchURL)
		if err != nil {
			return fmt.Errorf("google: %w", err)
		}

		found := 0
		matches := googleHrefRegex.FindAllSubmatch(body, -1)
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
			found++
			select {
			case results <- hostname:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if found == 0 {
			break
		}
		start += 100
	}
	return nil
}
