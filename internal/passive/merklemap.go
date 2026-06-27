package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strconv"
)

type merklemapResponse struct {
	Count   int `json:"count"`
	Results []struct {
		Hostname string `json:"hostname"`
	} `json:"results"`
}

type MerkleMap struct {
	keys []string
}

func (m *MerkleMap) Name() string          { return "MERKLEMAP_API_KEY" }
func (m *MerkleMap) NeedsKey() bool        { return true }
func (m *MerkleMap) SetKeys(keys []string) { m.keys = keys }

func (m *MerkleMap) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(m.keys) == 0 {
		return nil
	}
	apiKey := m.keys[rand.Intn(len(m.keys))]

	baseURL := "https://api.merklemap.com/v1/search?query=" + url.QueryEscape("*."+domain)
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	totalCount := math.MaxInt
	processedResults := 0

	for page := 0; processedResults < totalCount; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		apiURL := baseURL + "&page=" + strconv.Itoa(page)
		body, err := fetchWithHeaders(ctx, apiURL, "GET", headers, nil)
		if err != nil {
			return fmt.Errorf("merklemap: %w", err)
		}

		var pageResp merklemapResponse
		if err := json.Unmarshal(body, &pageResp); err != nil {
			return fmt.Errorf("merklemap parse: %w", err)
		}

		if page == 0 {
			totalCount = pageResp.Count
		}

		if len(pageResp.Results) == 0 {
			break
		}

		for _, result := range pageResp.Results {
			select {
			case results <- result.Hostname:
			case <-ctx.Done():
				return ctx.Err()
			}
			processedResults++
		}
	}
	return nil
}
