package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type dnsDumpsterResponse struct {
	A []struct {
		Host string `json:"host"`
	} `json:"a"`
	Ns []struct {
		Host string `json:"host"`
	} `json:"ns"`
}

type DNSDumpster struct {
	keys []string
}

func (d *DNSDumpster) Name() string          { return "DNSDUMPSTER_API_KEY" }
func (d *DNSDumpster) NeedsKey() bool        { return true }
func (d *DNSDumpster) SetKeys(keys []string) { d.keys = keys }

func (d *DNSDumpster) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(d.keys) == 0 {
		return nil
	}
	apiKey := d.keys[rand.Intn(len(d.keys))]

	body, err := fetchWithHeader(ctx, fmt.Sprintf("https://api.dnsdumpster.com/domain/%s", domain), "X-API-Key", apiKey)
	if err != nil {
		return fmt.Errorf("dnsdumpster: %w", err)
	}

	var resp dnsDumpsterResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("dnsdumpster parse: %w", err)
	}

	for _, record := range append(resp.A, resp.Ns...) {
		select {
		case results <- record.Host:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
