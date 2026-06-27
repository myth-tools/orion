package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

type CertSpotter struct {
	keys []string
}

func (c *CertSpotter) Name() string          { return "CERTSPOTTER_API_KEY" }
func (c *CertSpotter) NeedsKey() bool        { return true }
func (c *CertSpotter) SetKeys(keys []string) { c.keys = keys }

type certspotterEntry struct {
	DNSNames []string `json:"dns_names"`
}

func (c *CertSpotter) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(c.keys) == 0 {
		return nil
	}

	apiKey := c.keys[rand.Intn(len(c.keys))]
	headers := map[string]string{"Authorization": "Bearer " + apiKey}

	url := fmt.Sprintf("https://api.certspotter.com/v1/issuances?domain=%s&include_subdomains=true&expand=dns_names", domain)
	body, err := fetchWithHeaders(ctx, url, "GET", headers, nil)
	if err != nil {
		return fmt.Errorf("certspotter: %w", err)
	}

	var entries []certspotterEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("certspotter parse: %w", err)
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		for _, name := range entry.DNSNames {
			name = strings.TrimSpace(name)
			if name == "" || !strings.HasSuffix(name, "."+domain) {
				continue
			}
			if strings.HasPrefix(name, "*.") {
				continue
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			select {
			case results <- name:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
