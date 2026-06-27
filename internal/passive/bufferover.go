package passive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

var errBufferOverResponse = errors.New("bufferover response error")

type bufferOverResponse struct {
	Meta struct {
		Errors []string `json:"errors"`
	} `json:"meta"`
	FDNSA   []string `json:"fdns_a"`
	RDNS    []string `json:"rdns"`
	Results []string `json:"results"`
}

type BufferOver struct {
	keys []string
}

func (b *BufferOver) Name() string          { return "BUFFEROVER_API_KEY" }
func (b *BufferOver) NeedsKey() bool        { return true }
func (b *BufferOver) SetKeys(keys []string) { b.keys = keys }

func (b *BufferOver) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(b.keys) == 0 {
		return nil
	}
	apiKey := b.keys[rand.Intn(len(b.keys))]

	body, err := fetchWithHeader(ctx, fmt.Sprintf("https://tls.bufferover.run/dns?q=.%s", domain), "x-api-key", apiKey)
	if err != nil {
		return fmt.Errorf("bufferover: %w", err)
	}

	var resp bufferOverResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("bufferover parse: %w", err)
	}

	if len(resp.Meta.Errors) > 0 {
		return fmt.Errorf("%w: %s", errBufferOverResponse, strings.Join(resp.Meta.Errors, ", "))
	}

	var subdomains []string
	if len(resp.FDNSA) > 0 {
		subdomains = append(subdomains, resp.FDNSA...)
		subdomains = append(subdomains, resp.RDNS...)
	} else if len(resp.Results) > 0 {
		subdomains = resp.Results
	}

	seen := make(map[string]bool)
	for _, sub := range subdomains {
		subs := NewExtractor(domain).Extract(sub)
		for _, s := range subs {
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			select {
			case results <- s:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
