package passive

import (
	"context"
	"encoding/json"
	"fmt"
)

type hudsonRockResponse struct {
	Data struct {
		EmployeesUrls []struct {
			URL string `json:"url"`
		} `json:"employees_urls"`
		ClientsUrls []struct {
			URL string `json:"url"`
		} `json:"clients_urls"`
	} `json:"data"`
}

type HudsonRock struct{}

func (h *HudsonRock) Name() string       { return "hudsonrock" }
func (h *HudsonRock) NeedsKey() bool     { return false }
func (h *HudsonRock) SetKeys(_ []string) {}

func (h *HudsonRock) Fetch(ctx context.Context, domain string, results chan<- string) error {
	body, err := fetch(ctx, fmt.Sprintf("https://cavalier.hudsonrock.com/api/json/v2/osint-tools/urls-by-domain?domain=%s", domain))
	if err != nil {
		return fmt.Errorf("hudsonrock: %w", err)
	}

	var resp hudsonRockResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("hudsonrock parse: %w", err)
	}

	seen := make(map[string]bool)
	records := resp.Data.EmployeesUrls
	records = append(records, resp.Data.ClientsUrls...)
	for _, record := range records {
		subs := NewExtractor(domain).Extract(record.URL)
		for _, sub := range subs {
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
	}
	return nil
}
