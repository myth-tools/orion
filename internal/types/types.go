package types

import "time"

type Config struct {
	Domain           string                    `json:"domain,omitempty"`
	DomainsFile      string                    `json:"domains_file,omitempty"`
	Threads          int                       `json:"threads,omitempty"`
	Timeout          int                       `json:"timeout,omitempty"`
	OutputFile       string                    `json:"output_file,omitempty"`
	OutputDirectory  string                    `json:"output_directory,omitempty"`
	Silent           bool                      `json:"silent,omitempty"`
	Verbose          bool                      `json:"verbose,omitempty"`
	JSON             bool                      `json:"json,omitempty"`
	HostIP           bool                      `json:"host_ip,omitempty"`
	CaptureSources   bool                      `json:"capture_sources,omitempty"`
	RemoveWildcard   bool                      `json:"remove_wildcard,omitempty"`
	Statistics       bool                      `json:"statistics,omitempty"`
	ListSources      bool                      `json:"list_sources,omitempty"`
	Bruteforce       bool                      `json:"bruteforce,omitempty"`
	Wordlist         string                    `json:"wordlist,omitempty"`
	Resolvers        []string                  `json:"resolvers,omitempty"`
	PassiveOnly      bool                      `json:"passive_only,omitempty"`
	Permute          bool                      `json:"permute,omitempty"`
	PermuteLevel     int                       `json:"permute_level,omitempty"`
	NSECWalk         bool                      `json:"nsec_walk,omitempty"`
	DoH              bool                      `json:"doh,omitempty"`
	MaxWordlistSize  int                       `json:"max_wordlist_size,omitempty"`
	ProxyURL         string                    `json:"proxy_url,omitempty"`
	TorMode          bool                      `json:"tor_mode,omitempty"`
	ProviderConfig   string                    `json:"provider_config,omitempty"`
	Sources          []string                  `json:"sources,omitempty"`
	ExcludeSources   []string                  `json:"exclude_sources,omitempty"`
	Match            []string                  `json:"match,omitempty"`
	Filter           []string                  `json:"filter,omitempty"`
	RateLimit        int                       `json:"rate_limit,omitempty"`
	RateLimits       map[string]RateLimitEntry `json:"rate_limits,omitempty"`
	DNSPersecond     int                       `json:"dns_per_second,omitempty"`
	DNSRetries       int                       `json:"dns_retries,omitempty"`
	DNSTimeout       int                       `json:"dns_timeout,omitempty"`
	ActiveTimeout    int                       `json:"active_timeout,omitempty"`
	ProxyPool        bool                      `json:"proxy_pool,omitempty"`
	ProxyPoolMin     int                       `json:"proxy_pool_min,omitempty"`
	ProxyPoolRefresh int                       `json:"proxy_pool_refresh,omitempty"`
}

type RateLimitEntry struct {
	MaxCount uint          `json:"max_count"`
	Duration time.Duration `json:"duration"`
}

type DNSStat struct {
	Total     int
	Completed int
	Found     int
	Errors    int
	Timeouts  int
	Retries   int
	StartedAt time.Time
}

type SourceStat struct {
	Name      string        `json:"name"`
	Results   int           `json:"results"`
	Errors    int           `json:"errors"`
	TimeTaken time.Duration `json:"time_taken"`
	Requests  int           `json:"requests"`
	Skipped   bool          `json:"skipped"`
}
