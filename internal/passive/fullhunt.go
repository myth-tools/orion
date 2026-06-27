package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type fullHuntResponse struct {
	Hosts   []string `json:"hosts"`
	Message string   `json:"message"`
	Status  int      `json:"status"`
}

type FullHunt struct {
	keys []string
}

func (f *FullHunt) Name() string          { return "FULLHUNT_API_KEY" }
func (f *FullHunt) NeedsKey() bool        { return true }
func (f *FullHunt) SetKeys(keys []string) { f.keys = keys }

//nolint:dupl
func (f *FullHunt) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(f.keys) == 0 {
		return nil
	}
	apiKey := f.keys[rand.Intn(len(f.keys))]

	body, err := fetchWithHeader(ctx, fmt.Sprintf("https://fullhunt.io/api/v1/domain/%s/subdomains", domain), "X-API-KEY", apiKey)
	if err != nil {
		return fmt.Errorf("fullhunt: %w", err)
	}

	var resp fullHuntResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("fullhunt parse: %w", err)
	}

	for _, host := range resp.Hosts {
		select {
		case results <- host:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
