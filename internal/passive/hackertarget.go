package passive

import (
	"context"
	"fmt"
	"strings"
)

type Hackertarget struct{}

func (h *Hackertarget) Name() string       { return "hackertarget" }
func (h *Hackertarget) NeedsKey() bool     { return false }
func (h *Hackertarget) SetKeys(_ []string) {}

func (h *Hackertarget) Fetch(ctx context.Context, domain string, results chan<- string) error {
	url := fmt.Sprintf("https://api.hackertarget.com/hostsearch/?q=%s", domain)

	body, err := fetch(ctx, url)
	if err != nil {
		return fmt.Errorf("hackertarget: %w", err)
	}

	lines := strings.SplitSeq(string(body), "\n")
	for line := range lines {
		parts := strings.SplitN(line, ",", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" || !strings.Contains(name, ".") {
			continue
		}
		select {
		case results <- name:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
