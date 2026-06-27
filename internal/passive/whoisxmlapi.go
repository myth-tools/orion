package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
)

type whoisXMLResponse struct {
	Search string         `json:"search"`
	Result whoisXMLResult `json:"result"`
}

type whoisXMLResult struct {
	Count   int              `json:"count"`
	Records []whoisXMLRecord `json:"records"`
}

type whoisXMLRecord struct {
	Domain    string `json:"domain"`
	FirstSeen int    `json:"first_seen"`
	LastSeen  int    `json:"last_seen"`
}

type WhoisXMLAPI struct {
	keys []string
}

func (w *WhoisXMLAPI) Name() string          { return "WHOISXML_API_KEY" }
func (w *WhoisXMLAPI) NeedsKey() bool        { return true }
func (w *WhoisXMLAPI) SetKeys(keys []string) { w.keys = keys }

func (w *WhoisXMLAPI) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(w.keys) == 0 {
		return nil
	}
	apiKey := w.keys[rand.Intn(len(w.keys))]

	body, err := fetch(ctx, fmt.Sprintf("https://subdomains.whoisxmlapi.com/api/v1?apiKey=%s&domainName=%s", apiKey, domain))
	if err != nil {
		return fmt.Errorf("whoisxmlapi: %w", err)
	}

	var resp whoisXMLResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("whoisxmlapi parse: %w", err)
	}

	for _, record := range resp.Result.Records {
		select {
		case results <- record.Domain:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
