package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type ThreatMiner struct{}

func (t *ThreatMiner) Name() string       { return "threatminer" }
func (t *ThreatMiner) NeedsKey() bool     { return false }
func (t *ThreatMiner) SetKeys(_ []string) {}

type threatMinerResponse struct {
	StatusCode string   `json:"status_code"`
	Results    []string `json:"results"`
}

func (t *ThreatMiner) Fetch(ctx context.Context, domain string, results chan<- string) error {
	url := fmt.Sprintf("https://api.threatminer.org/v2/domain.php?q=%s&rt=5", domain)
	body, err := fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("threatminer: %w", err)
	}

	var resp threatMinerResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("threatminer parse: %w", err)
	}
	if resp.StatusCode != "200" { //nolint:usestdlibvars // API returns string, not int status
		return nil
	}

	seen := make(map[string]bool)
	for _, name := range resp.Results {
		name = strings.TrimSpace(name)
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
