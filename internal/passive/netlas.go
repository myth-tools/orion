package passive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
)

type netlasItem struct {
	Data struct {
		Domain string `json:"domain"`
	} `json:"data"`
}

type netlasCountResponse struct {
	Count int `json:"count"`
}

const netlasDownloadCap = 200

type Netlas struct {
	keys []string
}

func (n *Netlas) Name() string          { return "NETLAS_API_KEY" }
func (n *Netlas) NeedsKey() bool        { return true }
func (n *Netlas) SetKeys(keys []string) { n.keys = keys }

func (n *Netlas) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(n.keys) == 0 {
		return nil
	}
	apiKey := n.keys[rand.Intn(len(n.keys))]

	client := getHTTPClient(ctx)
	if client == nil {
		return errSharedClientNotSet
	}

	// Get count
	countQuery := fmt.Sprintf("domain:*.%s AND NOT domain:%s", domain, domain)
	countURL := "https://app.netlas.io/api/domains_count/?" + url.Values{"q": {countQuery}}.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, countURL, nil)
	req.Header.Set("User-Agent", randomUA())
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("netlas count: %w", err)
	}
	countBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var countResp netlasCountResponse
	if err = json.Unmarshal(countBody, &countResp); err != nil {
		return fmt.Errorf("netlas count parse: %w", err)
	}

	// Download domains
	query := fmt.Sprintf("domain:*.%s AND NOT domain:%s", domain, domain)
	reqBody := map[string]any{
		"q":           query,
		"fields":      []string{"*"},
		"source_type": "include",
		"size":        min(countResp.Count, netlasDownloadCap),
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("netlas marshal: %w", err)
	}

	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://app.netlas.io/api/domains/download/", bytes.NewReader(jsonBody))
	req2.Header.Set("User-Agent", randomUA())
	req2.Header.Set("X-Api-Key", apiKey)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")

	resp2, err := client.Do(req2)
	if err != nil {
		return fmt.Errorf("netlas download: %w", err)
	}
	defer resp2.Body.Close()

	body, err := io.ReadAll(resp2.Body)
	if err != nil {
		return fmt.Errorf("netlas read: %w", err)
	}

	var data []netlasItem
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("netlas parse: %w", err)
	}

	for _, item := range data {
		select {
		case results <- item.Data.Domain:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
