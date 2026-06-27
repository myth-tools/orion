package passive

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

var rapidDNSPagePattern = regexp.MustCompile(`class="page-link" href="/subdomain/[^"]+\?page=(\d+)">`)

type RapidDNS struct{}

func (r *RapidDNS) Name() string       { return "rapiddns" }
func (r *RapidDNS) NeedsKey() bool     { return false }
func (r *RapidDNS) SetKeys(_ []string) {}

func (r *RapidDNS) Fetch(ctx context.Context, domain string, results chan<- string) error {
	page := 1
	maxPages := 1
	seen := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		body, err := fetch(ctx, fmt.Sprintf("https://rapiddns.io/subdomain/%s?page=%d&full=1", domain, page))
		if err != nil {
			return fmt.Errorf("rapiddns: %w", err)
		}

		src := string(body)
		extractor := NewExtractor(domain)
		for _, sub := range extractor.Extract(src) {
			if sub == "" || seen[sub] {
				continue
			}
			seen[sub] = true
			select {
			case results <- sub:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if maxPages == 1 {
			matches := rapidDNSPagePattern.FindAllStringSubmatch(src, -1)
			if len(matches) > 0 {
				lastMatch := matches[len(matches)-1]
				if len(lastMatch) > 1 {
					maxPages, _ = strconv.Atoi(lastMatch[1])
				}
			}
		}

		if page >= maxPages {
			break
		}
		page++
	}
	return nil
}
