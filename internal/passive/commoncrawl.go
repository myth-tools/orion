package passive

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var errCommonCrawlNoCollections = errors.New("commoncrawl: no collections found")

type CommonCrawl struct{}

func (c *CommonCrawl) Name() string       { return "commoncrawl" }
func (c *CommonCrawl) NeedsKey() bool     { return false }
func (c *CommonCrawl) SetKeys(_ []string) {}

type collInfo struct {
	CDXAPI string `json:"cdx-api"` //nolint:tagliatelle // actual JSON field from API
}

type ccEntry struct {
	URL string `json:"url"`
}

func processCommonCrawlCollection(ctx context.Context, col collInfo, domain string, seen map[string]bool, results chan<- string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	apiURL := fmt.Sprintf("%s?url=*.%s&output=json&fl=url&limit=100000",
		col.CDXAPI, domain)

	crawlBody, err := fetch(ctx, apiURL)
	if err != nil {
		return fmt.Errorf("commoncrawl collection fetch: %w", err)
	}
	if len(crawlBody) == 0 {
		return nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(crawlBody))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry ccEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.URL == "" {
			continue
		}
		parsed, err := url.Parse(entry.URL)
		if err != nil {
			continue
		}
		hostname := strings.ToLower(parsed.Hostname())
		if hostname == "" || !strings.HasSuffix(hostname, "."+domain) || hostname == domain {
			continue
		}
		if seen[hostname] {
			continue
		}
		seen[hostname] = true
		select {
		case results <- hostname:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (c *CommonCrawl) Fetch(ctx context.Context, domain string, results chan<- string) error {
	collectionsURL := "https://index.commoncrawl.org/collinfo.json"
	body, err := fetch(ctx, collectionsURL)
	if err != nil {
		return fmt.Errorf("commoncrawl collinfo: %w", err)
	}

	var collections []collInfo
	if err := json.Unmarshal(body, &collections); err != nil {
		return fmt.Errorf("commoncrawl collinfo parse: %w", err)
	}
	if len(collections) == 0 {
		return errCommonCrawlNoCollections
	}

	seen := make(map[string]bool)
	limit := min(2, len(collections))
	for _, col := range collections[:limit] {
		if err := processCommonCrawlCollection(ctx, col, domain, seen, results); err != nil {
			return err
		}
	}
	return nil
}
