package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

type zoomeyeResults struct {
	Status int `json:"status"`
	Total  int `json:"total"`
	List   []struct {
		Name string   `json:"name"`
		IP   []string `json:"ip"`
	} `json:"list"`
}

type ZoomEyeAPI struct {
	keys []string
}

func (z *ZoomEyeAPI) Name() string          { return "ZOOMEYE_API_KEY" }
func (z *ZoomEyeAPI) NeedsKey() bool        { return true }
func (z *ZoomEyeAPI) SetKeys(keys []string) { z.keys = keys }

const defaultZoomEyeHost = "zoomeye.ai"

func (z *ZoomEyeAPI) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(z.keys) == 0 {
		return nil
	}
	rawKey := z.keys[rand.Intn(len(z.keys))]
	host := defaultZoomEyeHost
	apiKey := rawKey
	if parts := strings.SplitN(rawKey, ":", 2); len(parts) == 2 {
		host = parts[0]
		apiKey = parts[1]
	}

	headers := map[string]string{
		"API-KEY":      apiKey,
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	pages := 1
	currentPage := 1
	for ; currentPage <= pages; currentPage++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		apiURL := fmt.Sprintf("https://api.%s/domain/search?q=%s&type=1&s=1000&page=%d", host, domain, currentPage)

		body, err := fetchWithHeaders(ctx, apiURL, "GET", headers, nil)
		if err != nil {
			return fmt.Errorf("zoomeye: %w", err)
		}

		var res zoomeyeResults
		if err := json.Unmarshal(body, &res); err != nil {
			return fmt.Errorf("zoomeye parse: %w", err)
		}

		pages = res.Total/1000 + 1

		for _, r := range res.List {
			select {
			case results <- r.Name:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
