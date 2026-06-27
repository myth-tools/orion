package passive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type intelxSearchResponse struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
}

type intelxSelector struct {
	SelectValue string `json:"selectorvalue"`
}

type intelxSearchResult struct {
	Selectors []intelxSelector `json:"selectors"`
	Status    int              `json:"status"`
}

type intelxRequestBody struct {
	Term        string   `json:"term"`
	Maxresults  int      `json:"maxresults"`
	Media       int      `json:"media"`
	Terminate   []int    `json:"terminate"`
	Timeout     int      `json:"timeout"`
	Buckets     []string `json:"buckets"`
	Lookuplevel int      `json:"lookuplevel"`
	Sort        int      `json:"sort"`
}

type IntelX struct {
	keys []string
}

func (i *IntelX) Name() string          { return "INTELX_API_KEY" }
func (i *IntelX) NeedsKey() bool        { return true }
func (i *IntelX) SetKeys(keys []string) { i.keys = keys }

func (i *IntelX) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(i.keys) == 0 {
		return nil
	}

	// IntelX requires a compound key "host:key" format
	rawKey := i.keys[rand.Intn(len(i.keys))]
	parts := strings.SplitN(rawKey, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	host, apiKey := parts[0], parts[1]

	client := getHTTPClient(ctx)
	if client == nil {
		return errSharedClientNotSet
	}

	searchID, err := searchIntelX(ctx, client, host, apiKey, domain)
	if err != nil {
		return err
	}

	resultsURL := fmt.Sprintf("https://%s/intelligent/search/result?id=%s&limit=10000", host, searchID)
	status := 0

	for status == 0 || status == 3 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultsURL, nil)
		if err != nil {
			return fmt.Errorf("intelx result req: %w", err)
		}
		req.Header.Set("X-Key", apiKey)
		req.Header.Set("User-Agent", randomUA())

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("intelx result: %w", err)
		}

		var result intelxSearchResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return fmt.Errorf("intelx result parse: %w", err)
		}
		resp.Body.Close()

		status = result.Status
		for _, sel := range result.Selectors {
			select {
			case results <- sel.SelectValue:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if status != 0 && status != 3 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

func searchIntelX(ctx context.Context, client *http.Client, host, apiKey, domain string) (string, error) {
	reqBody := intelxRequestBody{
		Term:        domain,
		Maxresults:  10000,
		Media:       0,
		Timeout:     20,
		Buckets:     []string{},
		Lookuplevel: 0,
		Sort:        4,
		Terminate:   []int{},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("intelx marshal: %w", err)
	}

	searchURL := fmt.Sprintf("https://%s/intelligent/search", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("intelx req: %w", err)
	}
	req.Header.Set("X-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", randomUA())

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("intelx: %w", err)
	}

	var searchResp intelxSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("intelx parse: %w", err)
	}
	resp.Body.Close()

	return searchResp.ID, nil
}
