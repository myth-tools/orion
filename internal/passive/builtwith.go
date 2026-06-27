package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type builtwithResponse struct {
	Results []struct {
		Result struct {
			Paths []struct {
				Domain    string `json:"domain"`
				SubDomain string `json:"sub_domain"`
			} `json:"paths"`
		} `json:"result"`
	} `json:"results"`
}

type BuiltWith struct {
	keys []string
}

func (b *BuiltWith) Name() string          { return "BUILTWITH_API_KEY" }
func (b *BuiltWith) NeedsKey() bool        { return true }
func (b *BuiltWith) SetKeys(keys []string) { b.keys = keys }

func (b *BuiltWith) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(b.keys) == 0 {
		return nil
	}
	apiKey := b.keys[rand.Intn(len(b.keys))]

	body, err := fetch(ctx, fmt.Sprintf(
		"https://api.builtwith.com/v21/api.json?KEY=%s&HIDETEXT=yes&HIDEDL=yes&NOLIVE=yes&NOMETA=yes&NOPII=yes&NOATTR=yes&LOOKUP=%s",
		apiKey, domain,
	))
	if err != nil {
		return fmt.Errorf("builtwith: %w", err)
	}

	var resp builtwithResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("builtwith parse: %w", err)
	}

	for _, r := range resp.Results {
		for _, p := range r.Result.Paths {
			sub := p.SubDomain + "." + p.Domain
			select {
			case results <- sub:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
