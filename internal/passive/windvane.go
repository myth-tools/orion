package passive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
)

type windvanePageInfo struct {
	Total     string `json:"total"`
	Count     string `json:"count"`
	TotalPage string `json:"total_page"`
}

type windvaneDomainEntry struct {
	Domain string `json:"domain"`
}

type windvaneData struct {
	List         []windvaneDomainEntry `json:"list"`
	PageResponse windvanePageInfo      `json:"page_response"`
}

type windvaneResponse struct {
	Code int          `json:"code"`
	Msg  string       `json:"msg"`
	Data windvaneData `json:"data"`
}

type WindVane struct {
	keys []string
}

func (w *WindVane) Name() string          { return "WINDVANE_API_KEY" }
func (w *WindVane) NeedsKey() bool        { return true }
func (w *WindVane) SetKeys(keys []string) { w.keys = keys }

func (w *WindVane) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(w.keys) == 0 {
		return nil
	}
	apiKey := w.keys[rand.Intn(len(w.keys))]

	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Api-Key":    apiKey,
	}

	page := 1
	count := 1000

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reqBody, marshalErr := json.Marshal(map[string]any{
			"domain": domain,
			"page_request": map[string]int{
				"page":  page,
				"count": count,
			},
		})

		if marshalErr != nil {
			return fmt.Errorf("windvane marshal: %w", marshalErr)
		}

		body, err := fetchWithHeaders(ctx,
			"https://windvane.lichoin.com/trpc.backendhub.public.WindvaneService/ListSubDomain",
			"POST", headers, bytes.NewReader(reqBody))
		if err != nil {
			return fmt.Errorf("windvane: %w", err)
		}

		var windvaneResp windvaneResponse
		if err = json.Unmarshal(body, &windvaneResp); err != nil {
			return fmt.Errorf("windvane parse: %w", err)
		}

		for _, record := range windvaneResp.Data.List {
			select {
			case results <- record.Domain:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		totalRecords, _ := strconv.Atoi(windvaneResp.Data.PageResponse.Total)
		recordsPerPage, _ := strconv.Atoi(windvaneResp.Data.PageResponse.Count)

		if (page-1)*recordsPerPage >= totalRecords {
			break
		}
		page++
	}
	return nil
}
