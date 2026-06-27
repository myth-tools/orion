package active

import (
	"fmt"
	"slices"
	"strings"
)

var permutePrefixes = []string{ //nolint:gochecknoglobals
	"dev", "stage", "staging", "test", "testing", "qa", "uat",
	"prod", "production", "preprod", "pre-prod",
	"api", "api2", "api3", "admin", "manage", "management",
	"beta", "alpha", "canary", "edge", "cdn",
	"app", "web", "mobile", "m", "www", "ww2",
	"secure", "portal", "dashboard", "panel", "console",
	"internal", "private", "corp", "external",
	"old", "new", "backup", "primary", "secondary",
	"us", "eu", "asia", "apac", "emea", "amer",
	"us-east", "us-west", "eu-west", "eu-central",
	"nyc", "ams", "lon", "fra", "sfo", "sgp", "blr", "syd",
	"01", "02", "03", "04", "05", "10", "20",
	"v2", "v3", "v4", "v5",
	"infra", "ops", "mon", "log", "metric", "alert",
	"git", "ci", "cd", "build", "deploy",
	"db", "cache", "queue", "worker", "batch",
	"auth", "session", "token", "sso",
	"static", "assets", "img", "assets",
	"docs", "wiki", "kb",
	"chat", "talk", "meet", "call",
	"status", "health", "monitor", "uptime",
	"stg", "prd", "dev2", "tst",
}

var permuteSuffixes = []string{ //nolint:gochecknoglobals
	"dev", "stage", "staging", "test", "qa", "uat",
	"prod", "production", "preprod", "pre-prod",
	"api", "admin", "app", "web", "portal",
	"beta", "alpha", "canary", "edge", "cdn",
	"internal", "external", "private", "public",
	"backend", "frontend", "service", "services",
	"backup", "old", "new", "archive", "legacy",
	"db", "cache", "queue", "worker",
	"auth", "session", "token",
	"ui", "ux", "webapp",
	"1", "2", "3", "4", "5",
	"us", "eu", "asia",
	"west", "east", "north", "south",
	"lb", "proxy", "gw", "gateway",
}

var permuteNumbers = []string{ //nolint:gochecknoglobals
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
	"11", "12", "13", "14", "15", "20", "21", "25", "30", "50", "99", "100",
	"01", "02", "03", "04", "05", "001", "002",
}

var envRegions = []string{ //nolint:gochecknoglobals
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"eu-west-1", "eu-west-2", "eu-central-1", "eu-north-1",
	"ap-southeast-1", "ap-southeast-2", "ap-northeast-1", "ap-south-1",
	"sa-east-1", "ca-central-1", "me-south-1", "af-south-1",
}

type Permutator struct {
	level         int
	maxCandidates int
}

func NewPermutator(level int) *Permutator {
	if level < 1 {
		level = 1
	}
	if level > 3 {
		level = 3
	}
	maxCand := maxCandidatesForLevel(level)
	return &Permutator{
		level:         level,
		maxCandidates: maxCand,
	}
}

func maxCandidatesForLevel(level int) int {
	switch level {
	case 1:
		return 10000
	case 2:
		return 50000
	default:
		return 200000
	}
}

func (p *Permutator) Generate(subdomains []string, domain string) []string {
	seen := make(map[string]bool)
	var candidates []string

	for _, sub := range subdomains {
		sub = strings.TrimSpace(sub)
		if sub == "" || !strings.HasSuffix(sub, "."+domain) {
			continue
		}

		base := sub[:len(sub)-len("."+domain)]
		if base == "" {
			continue
		}

		if strings.Contains(base, ".") {
			continue
		}

		p.addCandidate(domain, base, &candidates, seen)
	}

	if len(candidates) > p.maxCandidates {
		candidates = candidates[:p.maxCandidates]
	}
	slices.Sort(candidates)
	return candidates
}

func (p *Permutator) addCandidate(domain, base string, candidates *[]string, seen map[string]bool) {
	if p.level >= 1 {
		p.addPrefixPerms(base, domain, candidates, seen)
		p.addSuffixPerms(base, domain, candidates, seen)
	}

	if p.level >= 2 {
		p.addNumberPerms(base, domain, candidates, seen)
	}

	if p.level >= 3 {
		p.addRegionPerms(base, domain, candidates, seen)
		p.addSubSubPerms(base, domain, candidates, seen)
	}
}

func (p *Permutator) addPrefixPerms(base, domain string, candidates *[]string, seen map[string]bool) {
	for _, prefix := range permutePrefixes {
		perm := fmt.Sprintf("%s-%s.%s", prefix, base, domain)
		if !seen[perm] {
			seen[perm] = true
			*candidates = append(*candidates, perm)
		}
		perm2 := fmt.Sprintf("%s.%s", prefix+base, domain)
		if !seen[perm2] {
			seen[perm2] = true
			*candidates = append(*candidates, perm2)
		}
	}
}

func (p *Permutator) addSuffixPerms(base, domain string, candidates *[]string, seen map[string]bool) {
	for _, suffix := range permuteSuffixes {
		perm := fmt.Sprintf("%s-%s.%s", base, suffix, domain)
		if !seen[perm] {
			seen[perm] = true
			*candidates = append(*candidates, perm)
		}
	}
}

func (p *Permutator) addNumberPerms(base, domain string, candidates *[]string, seen map[string]bool) {
	for _, n := range permuteNumbers {
		perm := fmt.Sprintf("%s%s.%s", base, n, domain)
		if !seen[perm] {
			seen[perm] = true
			*candidates = append(*candidates, perm)
		}
		perm2 := fmt.Sprintf("%s-%s.%s", base, n, domain)
		if !seen[perm2] {
			seen[perm2] = true
			*candidates = append(*candidates, perm2)
		}
	}
}

func (p *Permutator) addRegionPerms(base, domain string, candidates *[]string, seen map[string]bool) {
	for _, region := range envRegions {
		perm := fmt.Sprintf("%s-%s.%s", base, region, domain)
		if !seen[perm] {
			seen[perm] = true
			*candidates = append(*candidates, perm)
		}
	}
}

func (p *Permutator) addSubSubPerms(base, domain string, candidates *[]string, seen map[string]bool) {
	for _, prefix := range permutePrefixes[:10] {
		for _, suffix := range permuteSuffixes[:10] {
			perm := fmt.Sprintf("%s-%s-%s.%s", prefix, base, suffix, domain)
			if !seen[perm] {
				seen[perm] = true
				*candidates = append(*candidates, perm)
			}
		}
	}
}
