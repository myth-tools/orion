package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type reconeerSubdomain struct {
	Subdomain string `json:"subdomain"`
}

type reconeerResponse struct {
	Subdomains []reconeerSubdomain `json:"subdomains"`
}

type Reconeer struct {
	keys []string
}

func (r *Reconeer) Name() string          { return "reconeer" }
func (r *Reconeer) NeedsKey() bool        { return true }
func (r *Reconeer) SetKeys(keys []string) { r.keys = keys }

func (r *Reconeer) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(r.keys) == 0 {
		return nil
	}

	headers := map[string]string{
		"Accept":    "application/json",
		"X-API-KEY": r.keys[rand.Intn(len(r.keys))],
	}

	body, err := fetchWithHeaders(ctx,
		fmt.Sprintf("https://www.reconeer.com/api/domain/%s", domain),
		"GET", headers, nil)
	if err != nil {
		return fmt.Errorf("reconeer: %w", err)
	}

	var response reconeerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("reconeer parse: %w", err)
	}

	for _, result := range response.Subdomains {
		select {
		case results <- result.Subdomain:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
