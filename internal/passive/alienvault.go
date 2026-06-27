package passive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
)

var errAlienVaultAPI = errors.New("alienvault API error")

type AlienVault struct {
	apiKeys []string
}

func (a *AlienVault) Name() string          { return "ALIENVAULT_API_KEY" }
func (a *AlienVault) NeedsKey() bool        { return true }
func (a *AlienVault) SetKeys(keys []string) { a.apiKeys = keys }

type alienvaultResponse struct {
	Detail     string `json:"detail"`
	Error      string `json:"error"`
	PassiveDNS []struct {
		Hostname string `json:"hostname"`
	} `json:"passive_dns"`
}

func (a *AlienVault) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(a.apiKeys) == 0 {
		return nil
	}
	apiKey := a.apiKeys[rand.Intn(len(a.apiKeys))]

	url := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/passive_dns", domain)
	body, err := fetchWithHeaders(ctx, url, "GET",
		map[string]string{"Authorization": "Bearer " + apiKey}, nil)
	if err != nil {
		return fmt.Errorf("alienvault: %w", err)
	}

	var resp alienvaultResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("alienvault parse: %w", err)
	}

	if resp.Error != "" {
		return fmt.Errorf("%w: %s, %s", errAlienVaultAPI, resp.Error, resp.Detail)
	}

	seen := make(map[string]bool)
	for _, pdns := range resp.PassiveDNS {
		name := pdns.Hostname
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		select {
		case results <- name:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
