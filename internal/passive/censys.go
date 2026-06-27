package passive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
)

type censysSearchRequest struct {
	Query    string   `json:"query"`
	Fields   []string `json:"fields,omitempty"`
	PageSize int      `json:"page_size,omitempty"`
	Cursor   string   `json:"cursor,omitempty"`
}

type censysResponse struct {
	Result censysResult `json:"result"`
}

type censysResult struct {
	Hits          []censysHit `json:"hits"`
	NextPageToken string      `json:"next_page_token"`
}

type censysHit struct {
	CertificateV1 censysCertificateV1 `json:"certificate_v1"`
}

type censysCertificateV1 struct {
	Resource censysResource `json:"resource"`
}

type censysResource struct {
	Names []string `json:"names"`
}

type Censys struct {
	keys []string
}

func (c *Censys) Name() string          { return "CENSYS_API_KEY" }
func (c *Censys) NeedsKey() bool        { return true }
func (c *Censys) SetKeys(keys []string) { c.keys = keys }

func (c *Censys) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(c.keys) == 0 {
		return nil
	}

	pat := c.keys[rand.Intn(len(c.keys))]

	client := getHTTPClient(ctx)
	if client == nil {
		return errSharedClientNotSet
	}

	apiURL := "https://api.platform.censys.io/v3/global/search/query"
	cursor := ""
	currentPage := 0
	maxPages := 10

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reqBody := censysSearchRequest{
			Query:    "cert.names: " + domain,
			Fields:   []string{"cert.names"},
			PageSize: 100,
		}
		if cursor != "" {
			reqBody.Cursor = cursor
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("censys marshal: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("censys req: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+pat)
		req.Header.Set("User-Agent", randomUA())

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("censys: %w", err)
		}

		var censysResp censysResponse
		if err := json.NewDecoder(resp.Body).Decode(&censysResp); err != nil {
			resp.Body.Close()
			return fmt.Errorf("censys parse: %w", err)
		}
		resp.Body.Close()

		for _, hit := range censysResp.Result.Hits {
			for _, name := range hit.CertificateV1.Resource.Names {
				select {
				case results <- name:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}

		cursor = censysResp.Result.NextPageToken
		if cursor == "" || currentPage >= maxPages {
			break
		}
		currentPage++
	}
	return nil
}
