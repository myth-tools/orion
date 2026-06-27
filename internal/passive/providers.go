package passive

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Providers struct {
	AlienVault  []string `yaml:"ALIENVAULT_API_KEY,omitempty"`
	Bevigil     []string `yaml:"BEVIGIL_API_KEY,omitempty"`
	BufferOver  []string `yaml:"BUFFEROVER_API_KEY,omitempty"`
	Censys      []string `yaml:"CENSYS_API_KEY,omitempty"`
	CertSpotter []string `yaml:"CERTSPOTTER_API_KEY,omitempty"`
	Chaos       []string `yaml:"CHAOS_API_KEY,omitempty"`
	DNSDumpster []string `yaml:"DNSDUMPSTER_API_KEY,omitempty"`
	FullHunt    []string `yaml:"FULLHUNT_API_KEY,omitempty"`
	GitHub      []string `yaml:"GITHUB_API_KEY,omitempty"`
	IntelX      []string `yaml:"INTELX_API_KEY,omitempty"`
	LeakIX      []string `yaml:"LEAKIX_API_KEY,omitempty"`
	MerkleMap   []string `yaml:"MERKLEMAP_API_KEY,omitempty"`
	Netlas      []string `yaml:"NETLAS_API_KEY,omitempty"`
	Reconeer    []string `yaml:"RECONEER_API_KEY,omitempty"`
	Rsecloud    []string `yaml:"RSECLOUD_API_KEY,omitempty"`
	ThreatBook  []string `yaml:"THREATBOOK_API_KEY,omitempty"`
	URLScan     []string `yaml:"URLSCAN_API_KEY,omitempty"`
	VirusTotal  []string `yaml:"VIRUSTOTAL_API_KEY,omitempty"`
	WhoisXMLAPI []string `yaml:"WHOISXML_API_KEY,omitempty"`
	WindVane    []string `yaml:"WINDVANE_API_KEY,omitempty"`
	ZoomEyeAPI  []string `yaml:"ZOOMEYE_API_KEY,omitempty"`
	BuiltWith   []string `yaml:"BUILTWITH_API_KEY,omitempty"`
	DigitalYama []string `yaml:"DIGITALYAMA_API_KEY,omitempty"`
}

// programName is set at build time via ldflags (see Makefile).
var programName = "orion"

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", programName, "provider-config.yaml")
}

func EnsureConfig(path string) {
	if path == "" {
		path = DefaultConfigPath()
	}
	if path == "" {
		return
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
			return
		}
		_ = writeDefaultConfig(path)
	}
}

func LoadProviders(path string) (*Providers, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	if path == "" {
		return &Providers{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
				return nil, fmt.Errorf("creating config dir: %w", mkErr)
			}
			_ = writeDefaultConfig(path)
			return &Providers{}, nil
		}
		return nil, fmt.Errorf("reading provider config: %w", err)
	}

	var providers Providers
	if err := yaml.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("parsing provider config: %w", err)
	}
	return &providers, nil
}

// freeSources lists all built-in sources that need no API key.
var freeSources = []string{
	"crtsh", "wayback", "urlscan", "anubis",
	"commoncrawl", "bing", "baidu", "google", "threatminer",
	"shodanct", "rapiddns", "thc",
	"hudsonrock", "submd", "reconeer", "robtex", "hackertarget",
}

type sourceMeta struct {
	envVar string
	desc   string
	format string
}

var keyedSourcesMeta = []sourceMeta{
	{envVar: "ALIENVAULT_API_KEY", desc: "https://otx.alienvault.com (free account)"},
	{envVar: "BEVIGIL_API_KEY", desc: "https://bevigil.com"},
	{envVar: "BUFFEROVER_API_KEY", desc: "https://tls.bufferover.run"},
	{envVar: "BUILTWITH_API_KEY", desc: "https://builtwith.com"},
	{envVar: "CENSYS_API_KEY", desc: "https://censys.io"},
	{envVar: "CERTSPOTTER_API_KEY", desc: "https://certspotter.com (free account)"},
	{envVar: "CHAOS_API_KEY", desc: "https://chaos.projectdiscovery.io"},
	{envVar: "DIGITALYAMA_API_KEY", desc: "https://digitalyama.com"},
	{envVar: "DNSDUMPSTER_API_KEY", desc: "https://dnsdumpster.com"},
	{envVar: "FULLHUNT_API_KEY", desc: "https://fullhunt.io"},
	{envVar: "GITHUB_API_KEY", desc: "https://github.com/settings/tokens (classic, no scopes needed)"},
	{envVar: "INTELX_API_KEY", desc: "https://intelx.io", format: "format \"host:key\" — free: free.intelx.io, paid: 2.intelx.io"},
	{envVar: "LEAKIX_API_KEY", desc: "https://leakix.net"},
	{envVar: "MERKLEMAP_API_KEY", desc: "https://merklemap.com"},
	{envVar: "NETLAS_API_KEY", desc: "https://netlas.io"},
	{envVar: "RSECLOUD_API_KEY", desc: "https://rsecloud.com"},
	{envVar: "THREATBOOK_API_KEY", desc: "https://threatbook.cn"},
	{envVar: "VIRUSTOTAL_API_KEY", desc: "https://virustotal.com"},
	{envVar: "WHOISXML_API_KEY", desc: "https://whoisxmlapi.com"},
	{envVar: "WINDVANE_API_KEY", desc: "https://windvane.lichoin.com (100 free queries)"},
	{envVar: "ZOOMEYE_API_KEY", desc: "https://zoomeye.ai", format: "just your API key; or \"host:key\" for non-default instance"},
}

func generateDefaultConfig() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s provider config\n", programName))
	b.WriteString("# Fill in your API keys for the sources you want to use.\n")
	b.WriteString("# Format: SOURCE_API_KEY: [\"key1\", \"key2\"]\n")
	b.WriteString("# Multi-part keys (host:key, email:secret, etc.) use colon-separated values.\n")
	b.WriteString("\n")
	b.WriteString("# --- Free sources (no key needed) ---\n")
	b.WriteString("# " + strings.Join(freeSources, ", ") + "\n")
	b.WriteString("\n")
	b.WriteString("# --- Sources requiring API keys ---\n")

	sort.Slice(keyedSourcesMeta, func(i, j int) bool {
		return keyedSourcesMeta[i].envVar < keyedSourcesMeta[j].envVar
	})

	for _, m := range keyedSourcesMeta {
		if m.desc != "" {
			b.WriteString(fmt.Sprintf("# %s: %s\n", m.envVar, m.desc))
		}
		if m.format != "" {
			b.WriteString(fmt.Sprintf("#   %s\n", m.format))
		}
		b.WriteString(fmt.Sprintf("%s: []\n", m.envVar))
	}

	return b.String()
}

func writeDefaultConfig(path string) error {
	return os.WriteFile(path, []byte(generateDefaultConfig()), 0o644)
}
