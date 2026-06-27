package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type leakIXSubResponse struct {
	Subdomain   string    `json:"subdomain"`
	DistinctIps int       `json:"distinct_ips"`
	LastSeen    time.Time `json:"last_seen"`
}

type LeakIX struct {
	keys []string
}

func (l *LeakIX) Name() string          { return "LEAKIX_API_KEY" }
func (l *LeakIX) NeedsKey() bool        { return true } // API now requires authentication
func (l *LeakIX) SetKeys(keys []string) { l.keys = keys }

func (l *LeakIX) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(l.keys) == 0 {
		return nil
	}
	headers := map[string]string{
		"accept":  "application/json",
		"api-key": l.keys[rand.Intn(len(l.keys))],
	}

	body, err := fetchWithHeaders(ctx, fmt.Sprintf("https://leakix.net/api/subdomains/%s", domain), "GET", headers, nil)
	if err != nil {
		return fmt.Errorf("leakix: %w", err)
	}

	var subdomains []leakIXSubResponse
	if err := json.Unmarshal(body, &subdomains); err != nil {
		return fmt.Errorf("leakix parse: %w", err)
	}

	for _, result := range subdomains {
		select {
		case results <- result.Subdomain:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
