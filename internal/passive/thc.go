package passive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type thcResponse struct {
	Domains []struct {
		Domain string `json:"domain"`
	} `json:"domains"`
	NextPageState string `json:"next_page_state"`
}

type thcRequestBody struct {
	Domain    string `json:"domain"`
	PageState string `json:"page_state"`
	Limit     int    `json:"limit"`
}

type THC struct{}

func (t *THC) Name() string       { return "thc" }
func (t *THC) NeedsKey() bool     { return false }
func (t *THC) SetKeys(_ []string) {}

func (t *THC) Fetch(ctx context.Context, domain string, results chan<- string) error {
	client := getHTTPClient(ctx)
	if client == nil {
		return errSharedClientNotSet
	}

	var pageState string
	apiURL := "https://ip.thc.org/api/v1/lookup/subdomains"

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reqBody := thcRequestBody{
			Domain:    domain,
			PageState: pageState,
			Limit:     1000,
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("thc marshal: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("thc req: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", randomUA())

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("thc: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("thc: %w: %d", errHTTPStatus, resp.StatusCode)
		}

		rawBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("thc read: %w", err)
		}

		var tResp thcResponse
		if err := json.Unmarshal(rawBody, &tResp); err != nil {
			return fmt.Errorf("thc parse: %w", err)
		}

		for _, d := range tResp.Domains {
			select {
			case results <- d.Domain:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		pageState = tResp.NextPageState
		if pageState == "" {
			break
		}
	}
	return nil
}
