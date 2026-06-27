package passive

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

type gitHubItem struct {
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	TextMatches []struct {
		Fragment string `json:"fragment"`
	} `json:"text_matches"`
}

type gitHubResponse struct {
	TotalCount int          `json:"total_count"`
	Items      []gitHubItem `json:"items"`
}

type gitHubLink struct {
	URL string
	Rel string
}

type GitHub struct {
	keys []string
}

func (g *GitHub) Name() string          { return "GITHUB_API_KEY" }
func (g *GitHub) NeedsKey() bool        { return true }
func (g *GitHub) SetKeys(keys []string) { g.keys = keys }

func (g *GitHub) Fetch(ctx context.Context, domain string, results chan<- string) error {
	if len(g.keys) == 0 {
		return nil
	}

	apiKey := g.keys[rand.Intn(len(g.keys))]
	domainPattern := domainRegexpStr(domain)

	searchURL := fmt.Sprintf("https://api.github.com/search/code?per_page=100&q=%s&sort=created&order=asc", domain)
	return g.enumerate(ctx, searchURL, domainPattern, apiKey, results)
}

func (g *GitHub) enumerate(
	ctx context.Context, searchURL string,
	domainPattern *regexp.Regexp, apiKey string,
	results chan<- string,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	client := getHTTPClient(ctx)
	if client == nil {
		return errSharedClientNotSet
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return fmt.Errorf("github req: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3.text-match+json")
	req.Header.Set("Authorization", "token "+apiKey)
	req.Header.Set("User-Agent", randomUA())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("github read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// rate limit check
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			return nil
		}
		return fmt.Errorf("github: %w: %d", errHTTPStatus, resp.StatusCode)
	}

	var data gitHubResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("github parse: %w", err)
	}

	if err := g.processItems(ctx, data.Items, domainPattern, results); err != nil {
		return err
	}

	for _, link := range parseGitHubLinks(resp.Header.Get("Link")) {
		if link.Rel == "next" {
			if err := g.enumerate(ctx, link.URL, domainPattern, apiKey, results); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *GitHub) processItems(ctx context.Context, items []gitHubItem, domainPattern *regexp.Regexp, results chan<- string) error {
	var wg sync.WaitGroup

	for _, item := range items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go g.processItem(ctx, item, domainPattern, results, &wg)
	}

	wg.Wait()
	return nil
}

func (g *GitHub) processItem(
	ctx context.Context, item gitHubItem,
	domainPattern *regexp.Regexp, results chan<- string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	rawURL := rawGitHubURL(item.HTMLURL)
	if body, err := fetch(ctx, rawURL); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		for scanner.Scan() {
			line := gitHubNormalize(scanner.Text())
			g.sendMatches(ctx, domainPattern.FindAllString(line, -1), results)
		}
	}

	for _, tm := range item.TextMatches {
		g.sendMatches(ctx, domainPattern.FindAllString(gitHubNormalize(tm.Fragment), -1), results)
	}
}

func (g *GitHub) sendMatches(ctx context.Context, subs []string, results chan<- string) {
	for _, sub := range subs {
		select {
		case results <- sub:
		case <-ctx.Done():
			return
		}
	}
}

func domainRegexpStr(domain string) *regexp.Regexp {
	rdomain := strings.ReplaceAll(domain, ".", "\\.")
	return regexp.MustCompile("(\\w[a-zA-Z0-9][a-zA-Z0-9-\\.]*)" + rdomain)
}

func rawGitHubURL(htmlURL string) string {
	u := strings.ReplaceAll(htmlURL, "https://github.com/", "https://raw.githubusercontent.com/")
	return strings.ReplaceAll(u, "/blob/", "/")
}

func gitHubNormalize(content string) string {
	normalized, _ := url.QueryUnescape(content)
	normalized = strings.ReplaceAll(normalized, "\\t", "")
	normalized = strings.ReplaceAll(normalized, "\\n", "")
	return normalized
}

func parseGitHubLinks(header string) []gitHubLink {
	if header == "" {
		return nil
	}
	var links []gitHubLink
	for part := range strings.SplitSeq(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var link gitHubLink
		segments := strings.Split(part, ";")
		for i, seg := range segments {
			seg = strings.TrimSpace(seg)
			if i == 0 {
				seg = strings.TrimPrefix(seg, "<")
				seg = strings.TrimSuffix(seg, ">")
				link.URL = seg
			} else if after, ok := strings.CutPrefix(seg, `rel="`); ok {
				link.Rel = after
				link.Rel = strings.TrimSuffix(link.Rel, `"`)
			}
		}
		if link.URL != "" && link.Rel != "" {
			links = append(links, link)
		}
	}
	return links
}
