package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type bevigilResponse struct {
	Domain     string   `json:"domain"`
	Subdomains []string `json:"subdomains"`
}

type Bevigil struct {
	keys []string
}

func (b *Bevigil) Name() string          { return "BEVIGIL_API_KEY" }
func (b *Bevigil) NeedsKey() bool        { return true }
func (b *Bevigil) SetKeys(keys []string) { b.keys = keys }

//nolint:dupl
func (b *Bevigil) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(b.keys) == 0 {
		return nil
	}
	apiKey := b.keys[rand.Intn(len(b.keys))]

	body, err := fetchWithHeader(ctx,
		fmt.Sprintf("https://osint.bevigil.com/api/%s/subdomains/", domain),
		"X-Access-Token", apiKey)
	if err != nil {
		return fmt.Errorf("bevigil: %w", err)
	}

	var resp bevigilResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("bevigil parse: %w", err)
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
