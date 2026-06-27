package passive

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"strings"
)

type Submd struct {
	keys []string
}

func (s *Submd) Name() string          { return "submd" }
func (s *Submd) NeedsKey() bool        { return false }
func (s *Submd) SetKeys(keys []string) { s.keys = keys }

func (s *Submd) Fetch(ctx context.Context, domain string, results chan<- string) error {
	apiURL := fmt.Sprintf("https://api.sub.md/v1/search?apex=%s", domain)

	var body []byte
	var err error
	if len(s.keys) > 0 {
		body, err = fetchWithHeader(ctx, apiURL, "Authorization", "Bearer "+s.keys[rand.Intn(len(s.keys))])
	} else {
		body, err = fetch(ctx, apiURL)
	}
	if err != nil {
		return fmt.Errorf("submd: %w", err)
	}

	extractor := NewExtractor(domain)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		for _, sub := range extractor.Extract(line) {
			select {
			case results <- sub:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return scanner.Err()
}
