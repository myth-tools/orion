package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type rseCloudResponse struct {
	Data       []string `json:"data"`
	TotalPages int      `json:"total_pages"`
}

type RSECloud struct {
	keys []string
}

func (r *RSECloud) Name() string          { return "RSECLOUD_API_KEY" }
func (r *RSECloud) NeedsKey() bool        { return true }
func (r *RSECloud) SetKeys(keys []string) { r.keys = keys }

func (r *RSECloud) fetchEndpoint(ctx context.Context, endpoint, domain, apiKey string, results chan<- string) error {
	page := 1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		body, err := fetchWithHeaders(ctx,
			fmt.Sprintf("https://api.rsecloud.com/api/v2/subdomains/%s/%s?page=%d", endpoint, domain, page),
			"GET",
			map[string]string{
				"Content-Type": "application/json",
				"X-API-Key":    apiKey,
			},
			nil)
		if err != nil {
			return err
		}

		var resp rseCloudResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}

		for _, subdomain := range resp.Data {
			select {
			case results <- subdomain:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if page >= resp.TotalPages {
			break
		}
		page++
	}
	return nil
}

func (r *RSECloud) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(r.keys) == 0 {
		return nil
	}
	apiKey := r.keys[rand.Intn(len(r.keys))]

	if err := r.fetchEndpoint(ctx, "active", domain, apiKey, results); err != nil {
		return fmt.Errorf("rsecloud active: %w", err)
	}
	if err := r.fetchEndpoint(ctx, "passive", domain, apiKey, results); err != nil {
		return fmt.Errorf("rsecloud passive: %w", err)
	}
	return nil
}
