package passive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
)

var errThreatBookResponse = errors.New("threatbook response error")

type threatBookResponse struct {
	ResponseCode int64  `json:"response_code"`
	VerboseMsg   string `json:"verbose_msg"`
	Data         struct {
		SubDomains struct {
			Total string   `json:"total"`
			Data  []string `json:"data"`
		} `json:"sub_domains"`
	} `json:"data"`
}

type ThreatBook struct {
	keys []string
}

func (t *ThreatBook) Name() string          { return "THREATBOOK_API_KEY" }
func (t *ThreatBook) NeedsKey() bool        { return true }
func (t *ThreatBook) SetKeys(keys []string) { t.keys = keys }

func (t *ThreatBook) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(t.keys) == 0 {
		return nil
	}
	apiKey := t.keys[rand.Intn(len(t.keys))]

	body, err := fetch(ctx, fmt.Sprintf("https://api.threatbook.cn/v3/domain/sub_domains?apikey=%s&resource=%s", apiKey, domain))
	if err != nil {
		return fmt.Errorf("threatbook: %w", err)
	}

	var response threatBookResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("threatbook parse: %w", err)
	}

	if response.ResponseCode != 0 {
		return fmt.Errorf("%w: code %d, %s", errThreatBookResponse, response.ResponseCode, response.VerboseMsg)
	}

	total, err := strconv.ParseInt(response.Data.SubDomains.Total, 10, 64)
	if err != nil {
		return fmt.Errorf("threatbook total: %w", err)
	}

	if total > 0 {
		for _, subdomain := range response.Data.SubDomains.Data {
			select {
			case results <- subdomain:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
