package passive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

var errChaosStatus = errors.New("chaos status error")

type chaosResponse struct {
	Domain     string   `json:"domain"`
	Subdomains []string `json:"subdomains"`
}

type Chaos struct {
	keys []string
}

func (c *Chaos) Name() string          { return "CHAOS_API_KEY" }
func (c *Chaos) NeedsKey() bool        { return true }
func (c *Chaos) SetKeys(keys []string) { c.keys = keys }

func (c *Chaos) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(c.keys) == 0 {
		return nil
	}
	apiKey := c.keys[rand.Intn(len(c.keys))]

	client := getHTTPClient(ctx)
	if client == nil {
		return errSharedClientNotSet
	}

	apiURL := fmt.Sprintf("https://dns.projectdiscovery.io/dns/%s/subdomains", domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("chaos req: %w", err)
	}
	req.Header.Set("Authorization", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("chaos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", errChaosStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("chaos read: %w", err)
	}

	var chaosResp chaosResponse
	if err := json.Unmarshal(body, &chaosResp); err != nil {
		return fmt.Errorf("chaos parse: %w", err)
	}

	for _, sub := range chaosResp.Subdomains {
		select {
		case results <- sub + "." + domain:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
