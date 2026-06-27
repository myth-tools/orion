package passive

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type robtexResult struct {
	Rrname string `json:"rrname"`
	Rrdata string `json:"rrdata"`
	Rrtype string `json:"rrtype"`
}

type Robtex struct{}

func (r *Robtex) Name() string       { return "robtex" }
func (r *Robtex) NeedsKey() bool     { return false }
func (r *Robtex) SetKeys(_ []string) {}

func (r *Robtex) enumerate(ctx context.Context, url string) ([]robtexResult, error) {
	body, err := fetchWithHeaders(ctx, url, "GET", map[string]string{"Content-Type": "application/x-ndjson"}, nil)
	if err != nil {
		return nil, err
	}

	results := make([]robtexResult, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var result robtexResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, scanner.Err()
}

func (r *Robtex) Fetch(ctx context.Context, domain string, results chan<- string) error {
	baseURL := fmt.Sprintf("https://freeapi.robtex.com/pdns/forward/%s", domain)
	records, err := r.enumerate(ctx, baseURL)
	if err != nil {
		return fmt.Errorf("robtex forward: %w", err)
	}

	for _, record := range records {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if record.Rrtype == "A" || record.Rrtype == "AAAA" {
			reverseURL := fmt.Sprintf("https://freeapi.robtex.com/pdns/reverse/%s", record.Rrdata)
			domains, err := r.enumerate(ctx, reverseURL)
			if err != nil {
				return fmt.Errorf("robtex reverse: %w", err)
			}
			for _, d := range domains {
				select {
				case results <- d.Rrdata:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
	return nil
}
