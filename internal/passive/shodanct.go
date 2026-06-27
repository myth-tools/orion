package passive

import (
	"context"
	"encoding/json"
	"fmt"
)

type ShodanCT struct{}

func (s *ShodanCT) Name() string       { return "shodanct" }
func (s *ShodanCT) NeedsKey() bool     { return false }
func (s *ShodanCT) SetKeys(_ []string) {}

func (s *ShodanCT) Fetch(ctx context.Context, domain string, results chan<- string) error {
	body, err := fetch(ctx, fmt.Sprintf("https://ctl.shodan.io/api/v1/domain/%s/hostnames", domain))
	if err != nil {
		return fmt.Errorf("shodanct: %w", err)
	}

	var hostnames []string
	if err := json.Unmarshal(body, &hostnames); err != nil {
		return fmt.Errorf("shodanct parse: %w", err)
	}

	seen := make(map[string]bool)
	for _, hostname := range hostnames {
		subs := NewExtractor(domain).Extract(hostname)
		for _, sub := range subs {
			if sub == "" || seen[sub] {
				continue
			}
			seen[sub] = true
			select {
			case results <- sub:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
