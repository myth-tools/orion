package passive

import (
	"regexp"
	"strings"
)

type Extractor struct {
	pattern *regexp.Regexp
}

func NewExtractor(domain string) *Extractor {
	return &Extractor{
		pattern: regexp.MustCompile(`(?i)[a-zA-Z0-9\*_.-]+\.` + regexp.QuoteMeta(domain)),
	}
}

func (e *Extractor) Extract(text string) []string {
	matches := e.pattern.FindAllString(text, -1)
	for i, m := range matches {
		matches[i] = strings.ToLower(strings.TrimPrefix(m, "*."))
	}
	return matches
}
