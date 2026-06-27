package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type vtResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
	Meta struct {
		Cursor string `json:"cursor"`
	} `json:"meta"`
}

type VirusTotal struct {
	keys []string
}

func (v *VirusTotal) Name() string          { return "VIRUSTOTAL_API_KEY" }
func (v *VirusTotal) NeedsKey() bool        { return true }
func (v *VirusTotal) SetKeys(keys []string) { v.keys = keys }

func (v *VirusTotal) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(v.keys) == 0 {
		return nil
	}
	apiKey := v.keys[rand.Intn(len(v.keys))]

	cursor := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		url := fmt.Sprintf("https://www.virustotal.com/api/v3/domains/%s/subdomains?limit=40", domain)
		if cursor != "" {
			url = fmt.Sprintf("%s&cursor=%s", url, cursor)
		}

		body, err := fetchWithHeader(ctx, url, "x-apikey", apiKey)
		if err != nil {
			return fmt.Errorf("virustotal: %w", err)
		}

		var resp vtResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("virustotal parse: %w", err)
		}

		for _, d := range resp.Data {
			select {
			case results <- d.ID:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		cursor = resp.Meta.Cursor
		if cursor == "" {
			break
		}
	}
	return nil
}
