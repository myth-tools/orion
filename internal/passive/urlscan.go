package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type URLScan struct{}

func (u *URLScan) Name() string       { return "urlscan" }
func (u *URLScan) NeedsKey() bool     { return false }
func (u *URLScan) SetKeys(_ []string) {}

type urlscanResponse struct {
	Results []struct {
		Page struct {
			Domain string `json:"domain"`
			IP     string `json:"ip"`
			URL    string `json:"url"`
		} `json:"page"`
	} `json:"results"`
}

func (u *URLScan) Fetch(ctx context.Context, domain string, results chan<- string) error {
	apiURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=10000", domain)
	body, err := fetch(ctx, apiURL)
	if err != nil {
		return fmt.Errorf("urlscan: %w", err)
	}

	var resp urlscanResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("urlscan parse: %w", err)
	}

	seen := make(map[string]bool)
	for _, r := range resp.Results {
		name := strings.ToLower(r.Page.Domain)
		if name == "" || seen[name] || !strings.HasSuffix(name, "."+domain) {
			continue
		}

		if r.Page.URL != "" {
			parsed, err := url.Parse(r.Page.URL)
			if err == nil {
				hn := strings.ToLower(parsed.Hostname())
				if strings.HasSuffix(hn, "."+domain) && !seen[hn] {
					seen[hn] = true
					select {
					case results <- hn:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}

		seen[name] = true
		select {
		case results <- name:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
