package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type Wayback struct{}

func (w *Wayback) Name() string       { return "wayback" }
func (w *Wayback) NeedsKey() bool     { return false }
func (w *Wayback) SetKeys(_ []string) {}

func (w *Wayback) Fetch(ctx context.Context, domain string, results chan<- string) error {
	apiURL := fmt.Sprintf("https://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&fl=original&collapse=urlkey&limit=100000&from=2015",
		domain)
	body, err := fetch(ctx, apiURL)
	if err != nil {
		return fmt.Errorf("wayback: %w", err)
	}

	var rows [][]string
	if err := json.Unmarshal(body, &rows); err != nil {
		return fmt.Errorf("wayback parse: %w", err)
	}

	seen := make(map[string]bool)
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		rawURL := row[0]
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
			return ctx.Err()
		}
	}
	return nil
}
