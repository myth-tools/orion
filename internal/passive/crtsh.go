package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type CRTSH struct{}

func (c *CRTSH) Name() string       { return "crt.sh" }
func (c *CRTSH) NeedsKey() bool     { return false }
func (c *CRTSH) SetKeys(_ []string) {}

type crtshEntry struct {
	NameValue  string `json:"name_value"`
	CommonName string `json:"common_name"`
}

func (c *CRTSH) Fetch(ctx context.Context, domain string, results chan<- string) error {
	body, err := fetch(ctx, fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain))
	if err != nil {
		return fmt.Errorf("crt.sh: %w", err)
	}

	var entries []crtshEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("crt.sh parse: %w", err)
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		dnsNames := strings.Split(entry.NameValue, "\n")
		dnsNames = append(dnsNames, strings.Split(entry.CommonName, "\n")...)
		for _, name := range dnsNames {
			name = strings.TrimSpace(name)
			if name == "" || !strings.HasSuffix(name, "."+domain) && name != domain {
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
