package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type digitalYamaResponse struct {
	Subdomains []string `json:"subdomains"`
}

type DigitalYama struct {
	keys []string
}

func (d *DigitalYama) Name() string          { return "DIGITALYAMA_API_KEY" }
func (d *DigitalYama) NeedsKey() bool        { return true }
func (d *DigitalYama) SetKeys(keys []string) { d.keys = keys }

//nolint:dupl
func (d *DigitalYama) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(d.keys) == 0 {
		return nil
	}
	apiKey := d.keys[rand.Intn(len(d.keys))]

	body, err := fetchWithHeader(ctx,
		fmt.Sprintf("https://api.digitalyama.com/subdomain_finder?domain=%s", domain),
		"x-api-key", apiKey)
	if err != nil {
		return fmt.Errorf("digitalyama: %w", err)
	}

	var resp digitalYamaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("digitalyama parse: %w", err)
	}

	for _, sub := range resp.Subdomains {
		select {
		case results <- sub:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
