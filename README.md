<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0-blue?style=for-the-badge" alt="Version">
  <img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green?style=for-the-badge" alt="MIT License">
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows%20%7C%20android-lightgrey?style=for-the-badge" alt="Platforms">
  <img src="https://img.shields.io/badge/build-passing-brightgreen?style=for-the-badge" alt="Build">
  <img src="https://img.shields.io/badge/coverage-90%25-yellow?style=for-the-badge" alt="Coverage">
</p>

<div align="center">

<pre>
    ____  ____  ________  _   __
   / __ \/ __ \/  _/ __ \| | / /
  / / / / /_/ // // / / /  |/ /
 / /_/ / _, _// // /_/ / /|  /
 \____/_/ |_/___/\____/_/ |_/
</pre>

</div>

<p align="center">
  <strong>Enterprise-grade subdomain enumeration</strong><br>
  38 passive intelligence sources &bull; DNS brute-force &bull; 3-level permutation engine &bull; NSEC zone walking<br>
  Built-in rotating proxy pool &bull; Tor support &bull; 10 DoH providers &bull; Rate limiting &bull; System diagnostics
</p>

<p align="center">
  <a href="#-quick-start">Quick Start</a> &bull;
  <a href="#-features">Features</a> &bull;
  <a href="#-installation">Installation</a> &bull;
  <a href="#-usage">Usage</a> &bull;
  <a href="#-examples">Examples</a> &bull;
  <a href="#%EF%B8%8F-doctor-diagnostics">Doctor</a> &bull;
  <a href="#-architecture">Architecture</a> &bull;
  <a href="#-development">Development</a>
</p>

---

## Table of Contents

- [Why Orion?](#-why-orion)
- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Project Structure](#-project-structure)
- [Usage](#-usage)
  - [Required Arguments](#required-arguments)
  - [Output Control](#output-control)
  - [Output Format](#output-format)
  - [Source Selection](#source-selection)
  - [Discovery Techniques](#discovery-techniques)
  - [DNS & Performance](#dns--performance)
  - [Proxy & Anonymity](#proxy--anonymity)
  - [Rotating Proxy Pool](#rotating-proxy-pool)
  - [Rate Limiting](#rate-limiting)
  - [Meta Flags](#meta-flags)
  - [Subcommands](#subcommands)
- [Passive Sources](#-passive-sources)
- [Active Techniques](#-active-techniques)
- [Doctor Diagnostics](#%EF%B8%8F-doctor-diagnostics)
- [Output Formats](#-output-formats)
- [Exit Codes](#-exit-codes)
- [Architecture](#-architecture)
- [DNS-over-HTTPS](#-dns-over-https)
- [Configuration](#-configuration)
- [Performance & Tuning](#-performance--tuning)
- [Security & Ethics](#-security--ethics)
- [FAQ & Troubleshooting](#-faq--troubleshooting)
- [Build System](#%EF%B8%8F-build-system)
- [Development](#-development)
- [Examples](#-examples)
- [Contributing](#-contributing)
- [License](#-license)

---

## Why Orion?

> **Orion is not just another subdomain scraper.** It is a purpose-built, production-grade reconnaissance engine designed for security professionals, red teams, and researchers who need thorough, reliable, and anonymous subdomain enumeration.

| Capability | Orion | Typical Tools |
|---|---|---|
| Passive sources | **38** (17 free + 21 keyed) | 10–20 |
| Proxy pool sources | **18** (auto-scraped, tested, rotated) | None or manual |
| DoH providers | **10** (round-robin with fallback) | 1–3 |
| Permutation levels | **3** (prefix/suffix, leet/numbers, regions) | 1 level or none |
| NSEC zone walk | **Built-in** with proxy routing | Rarely supported |
| System diagnostics | **15+ check categories** (doctor) | None |
| Automatic 429 backoff | **Exponential + Retry-After** | Basic retry |
| Wildcard filtering | **Random probe + IP blocking** | Manual |
| Tor support | **Circuit isolation per request** | External tooling |
| Graceful shutdown | **Signal handler with flush** | Ctrl+C kill |
| Cross-platform | **Linux, macOS, Windows, Termux/Android** | Usually Linux only |

> **Bottom line:** If your workflow touches subdomain enumeration, Orion will find more, find it faster, and keep you anonymous doing it.

---

## Features

### Intelligence Gathering

- **38 passive sources** — certificate transparency logs (crt.sh, CertSpotter), search engines (Google, Bing, Baidu), DNS databases (SecurityTrails, AlienVault, ThreatMiner), threat intelligence (VirusTotal, LeakIX, IntelX), web archives (Wayback Machine, CommonCrawl), and more
- **Per-source rate limiting** with built-in defaults for every keyed source
- **Automatic 429 Retry-After** — exponential backoff with jitter when sources rate-limit you
- **Source exclusion/inclusion** — pick exactly which sources to use or skip
- **Glob match/filter patterns** — keep or discard subdomains by pattern

### Active Probing

- **DNS brute-force** — multi-threaded wordlist-based subdomain discovery with configurable rate limiting, retries, and smart stop conditions (error rate > 50%, spurious errors, timeout rate)
- **3-level permutation engine** — learns naming patterns from discovered subdomains and generates plausible variations
- **NSEC zone walking** — enumerates DNSSEC-enabled domains by walking the NSEC record chain
- **Wildcard detection & filtering** — random subdomain probes detect wildcard DNS; resolved wildcard IPs are filtered from all results
- **Custom wordlists** — bring your own wordlist or use the built-in ~5000 entry default

### Anonymity & Infrastructure

- **Built-in rotating proxy pool** — scrapes free HTTP/SOCKS5 proxies from 18 public lists, tests them concurrently (3s timeout), and rotates through working proxies in round-robin
- **SOCKS5 / Tor support** — route all traffic through a SOCKS5 proxy or Tor (127.0.0.1:9050) with per-request circuit rotation
- **10 DoH providers** — Cloudflare, Google, Quad9, OpenDNS, CleanBrowsing, AdGuard, Mullvad, ControlD, NextDNS, DNSForge — round-robin with automatic fallback
- **UDP DNS fallback** — when DoH is unavailable or disabled

### Output & Integration

- **Multiple output formats** — plain text, NDJSON, host-IP CSV (with `-ip`), source-annotated (with `-cs`)
- **Per-domain output files** — `-oD` flag creates separate result files per target domain
- **Source statistics** — per-source performance metrics after scan completion
- **Stdin support** — pipe domains for integration into automated workflows
- **Graceful signal handling** — SIGINT/Ctrl+C flushes all collected results; second Ctrl+C forces immediate exit
- **`-silent` mode** — emit only discovered subdomains, ideal for piping into other tools

### Reliability & Quality of Life

- **Doctor diagnostics** — 15+ category system verification covering Go environment, build, dependencies, passive sources, DoH, DNS, network, Tor, proxy pool, wordlist, permissions, and configuration validation
- **Smart stop conditions** — brute-force aborts early when error rate exceeds 50%, when spurious errors accumulate, or when timeout rate degrades
- **Version consistency enforcement** — single source of truth (`metadata.yaml`) verified by tests
- **Cross-platform builds** — Linux (amd64, 386, arm64, armv6, armv7), macOS (Intel, Apple Silicon), Windows (amd64, 386), Termux/Android (ARM64, amd64)

---

## Installation

### Go Install

> **Prerequisite:** Go 1.21+ (1.25+ recommended)

```sh
go install github.com/myth-tools/orion@latest
```

### Pre-built Binaries

Download the latest release from the [Releases page](https://github.com/myth-tools/orion/releases).

| Platform | Architecture | Archive |
|---|---|---|
| **Linux** | x86_64 | `orion_0.1.0_linux_amd64.tar.gz` |
| | x86 | `orion_0.1.0_linux_386.tar.gz` |
| | ARM64 | `orion_0.1.0_linux_arm64.tar.gz` |
| | ARMv7 | `orion_0.1.0_linux_armv7.tar.gz` |
| | ARMv6 | `orion_0.1.0_linux_armv6.tar.gz` |
| **macOS** | Intel | `orion_0.1.0_darwin_amd64.tar.gz` |
| | Apple Silicon | `orion_0.1.0_darwin_arm64.tar.gz` |
| **Windows** | x86_64 | `orion_0.1.0_windows_amd64.zip` |
| | x86 | `orion_0.1.0_windows_386.zip` |
| **Termux/Android** | ARM64 | `orion_0.1.0_termux_arm64.tar.gz` |
| | x86_64 | `orion_0.1.0_termux_amd64.tar.gz` |

<details>
<summary><strong>Extraction & installation commands</strong></summary>

```sh
# Linux / macOS
tar xzf orion_0.1.0_linux_amd64.tar.gz
sudo install orion /usr/local/bin/

# Windows (PowerShell)
Expand-Archive -Path orion_0.1.0_windows_amd64.zip -DestinationPath .
.\orion.exe -d example.com

# Termux / Android
tar xzf orion_0.1.0_termux_arm64.tar.gz
install orion $PREFIX/bin/
```

</details>

### Build from Source

```sh
git clone https://github.com/myth-tools/orion.git
cd orion
make build           # builds for current platform
sudo make install    # installs to $GOBIN or $GOPATH/bin
```

> For all supported platforms at once: `make build-all`

---

## Quick Start

> [!TIP]
> Run **`orion doctor`** before your first scan to verify every component is working.

```sh
# Diagnostics — verify your environment
orion doctor

# Basic scan (passive sources + DNS brute-force)
orion -d example.com

# Full deep scan — one flag: permute + NSEC + Tor + 60s timeout
orion -d example.com -deep

# Passive sources only (no contact with target DNS)
orion -d example.com -passive -o results.txt

# Batch scan multiple targets
orion -dL domains.txt -deep -silent

# JSON output piped to jq for processing
orion -d example.com -json | jq '.host'
```

---

## Project Structure

```
orion/
├── cmd/
│   └── orion/
│       ├── main.go                       # CLI entry point, flag parsing, scan orchestration
│       ├── help.go                       # Help text generation (all flag documentation)
│       ├── main_test.go                  # CLI flag and domain loading tests
│       └── version_consistency_test.go   # Version consistency enforcement tests
├── internal/
│   ├── active/
│   │   ├── bruteforce.go                 # DNS brute-force engine with rate limiting & retries
│   │   ├── nsecwalk.go                   # NSEC zone walking via DNSSEC records
│   │   ├── permutate.go                  # Permutation engine (3 levels)
│   │   ├── resolver.go                   # DNS resolver (DoH + UDP, wildcard detection)
│   │   └── transport.go                  # Proxy dialer, circuit breaker, HTTP client
│   ├── dns/
│   │   └── doh.go                        # DoH resolver with 10 providers, round-robin
│   ├── doctor/
│   │   └── doctor.go                     # System diagnostics (15+ check categories)
│   ├── passive/
│   │   ├── alienvault.go                 # AlienVault OTX
│   │   ├── anubis.go                     # Anubis
│   │   ├── bevigil.go                    # BeVigil
│   │   ├── bufferover.go                 # BufferOver
│   │   ├── builtwith.go                  # BuiltWith
│   │   ├── censys.go                     # Censys
│   │   ├── certspotter.go                # CertSpotter
│   │   ├── chaos.go                      # ProjectDiscovery Chaos
│   │   ├── commoncrawl.go                # CommonCrawl
│   │   ├── crtsh.go                      # crt.sh certificate transparency
│   │   ├── digitalyama.go                # DigitalYama
│   │   ├── dnsdumpster.go                # DNSDumpster
│   │   ├── doctorurls.go                 # Doctor URL test endpoints
│   │   ├── extractor.go                  # Domain extraction from text
│   │   ├── fetcher.go                    # HTTP client, rate limiting, retry, Source interface
│   │   ├── fullhunt.go                   # FullHunt
│   │   ├── github.go                     # GitHub code search
│   │   ├── google.go                     # Google search
│   │   ├── hackertarget.go               # HackerTarget
│   │   ├── hudsonrock.go                 # HudsonRock
│   │   ├── intelx.go                     # IntelX
│   │   ├── leakix.go                     # LeakIX
│   │   ├── merklemap.go                  # MerkleMap
│   │   ├── netlas.go                     # Netlas
│   │   ├── providers.go                  # Provider config loading, API key management
│   │   ├── rapiddns.go                   # RapidDNS
│   │   ├── ratelimit.go                  # Token-bucket rate limiter, MultiLimiter
│   │   ├── reconeer.go                   # Reconeer
│   │   ├── robtex.go                     # Robtex
│   │   ├── rsecloud.go                   # RSECloud
│   │   ├── searchengines.go              # Bing, Baidu, Google search engines
│   │   ├── shodanct.go                   # Shodan Certificate Transparency
│   │   ├── submd.go                      # SubMD
│   │   ├── thc.go                        # THC
│   │   ├── threatbook.go                 # ThreatBook
│   │   ├── threatminer.go                # ThreatMiner
│   │   ├── urlscan.go                    # URLScan.io
│   │   ├── virustotal.go                 # VirusTotal
│   │   ├── wayback.go                    # Wayback Machine CDX
│   │   ├── whoisxmlapi.go                # WhoisXML API
│   │   ├── windvane.go                   # WindVane
│   │   └── zoomeyeapi.go                 # ZoomEye API
│   ├── proxy/
│   │   ├── pool.go                       # Proxy pool manager (scrape, test, rotate, refresh)
│   │   ├── scraper.go                    # Proxy scraper (18 sources)
│   │   ├── tester.go                     # Proxy tester (HTTP + SOCKS5)
│   │   └── transport.go                  # Rotating transport for HTTP client
│   ├── runner/
│   │   ├── runner.go                     # Scan orchestrator (passive, active, output)
│   │   ├── output.go                     # Output writers (plain, JSON, host-IP, sources)
│   │   └── wordlist.go                   # Built-in default wordlist (~5000 entries)
│   ├── styler/
│   │   └── styler.go                     # Terminal styling (lipgloss-based)
│   └── types/
│       └── types.go                      # Shared types (Config, DNSStat, SourceStat)
├── scripts/
│   ├── lint.sh                           # Mega lint script (gofumpt, staticcheck, gosec, etc.)
│   └── release.sh                        # Automated release script (semver bump, build, publish)
├── .golangci.yml                         # golangci-lint configuration (100+ linters)
├── .gitignore                            # Comprehensive gitignore
├── go.mod                                # Go module definition
├── go.sum                                # Go module checksums
├── Makefile                              # Build, test, lint, release automation
├── metadata.yaml                         # Single source of truth (version, project metadata)
└── LICENSE                               # MIT License
```

---

## Usage

```
orion [FLAGS] -d <domain>
orion [FLAGS] -dL <domains.txt>
orion [FLAGS] -d <dom> -o <file>
orion doctor [FLAGS]
```

### Required Arguments

> One of the following is required.

| Flag | Description |
|---|---|
| `-d <domain>` | Target domain to enumerate (e.g., `example.com`) |
| `-dL <file>` | Path to a file with domains (one per line; lines starting with `#` are skipped) |

### Output Control

| Flag | Default | Description |
|---|---|---|
| `-o <file>` | `stdout` | Write results to file instead of stdout |
| `-oD <dir>` | `""` | Output directory — creates per-domain files |
| `-silent` | `false` | Suppress banner and progress; emit only discovered subdomains |
| `-v` | `false` | Verbose per-source logging and discovery progress |

### Output Format

| Flag | Default | Description |
|---|---|---|
| `-json` | `false` | NDJSON output — one JSON object per subdomain on stdout |
| `-ip` | `false` | Include resolved IPs in output (`host,ip,source` CSV format; implies `-nW`) |
| `-cs` | `false` | Show source name(s) that discovered each subdomain |
| `-stats` | `false` | Print per-source statistics after scan (results, errors, requests, duration) |
| `-nW` | `false` | Remove wildcard/dead subdomains via DNS resolution |

### Source Selection

| Flag | Default | Description |
|---|---|---|
| `-ls` | `false` | List all available passive sources and exit |
| `-provider-config <path>` | `~/.config/orion/provider-config.yaml` | Config file for API keys |
| `-sources <list>` | `all` | Comma-separated source names to use (e.g., `-sources crtsh,alienvault`) |
| `-es <list>` | `""` | Comma-separated source names to exclude |
| `-match <pattern>` | `""` | Glob patterns — only include matching subdomains |
| `-filter <pattern>` | `""` | Glob patterns — exclude matching subdomains |

### Discovery Techniques

| Flag | Default | Description |
|---|---|---|
| `-b` | `true` | DNS brute-force: test every word in wordlist against target |
| `-passive` | `false` | Disable active techniques; gather from 38 internet sources only |
| `-permute` | `false` | Learn naming patterns from found subdomains and generate variations |
| `-permute-level <n>` | `2` | Permutation depth: `1` = basic (prefix/suffix), `2` = aggressive (leet+swap), `3` = extreme (all combos) |
| `-nsec` | `false` | NSEC zone walk: query target's DNS for DNSSEC next-secure records |

### DNS & Performance

| Flag | Default | Description |
|---|---|---|
| `-r <list>` | `1.1.1.1:53,8.8.8.8:53,9.9.9.9:53` | Custom DNS resolvers, comma-separated (`host:port`) |
| `-doh` | `true` | Resolve via DNS-over-HTTPS (10 providers, round-robin); falls back to UDP if unavailable |
| `-timeout <n>` | `30` | Max seconds to wait per source HTTP request |
| `-t <n>` | `NumCPU×10` | Number of concurrent goroutines for brute-force and permutation |
| `-w <file>` | `""` | Custom wordlist path for brute-force; overrides embedded default (~5000 entries) |
| `-max-words <n>` | `0` (all) | Limit wordlist to first N entries |
| `-dns-rate <n>` | `0` (auto) | Max DNS lookups per second |
| `-dns-timeout <n>` | `5` | Per-request DNS timeout (seconds) |
| `-dns-retries <n>` | `2` | Retries per failed DNS lookup |
| `-active-timeout <n>` | `300` | Max seconds for brute-force + permutation phase |

### Proxy & Anonymity

| Flag | Default | Description |
|---|---|---|
| `-proxy <url>` | `""` | SOCKS5 proxy URL (e.g., `socks5://127.0.0.1:9050`) |
| `-tor` | `false` | Enable Tor mode: proxy defaults to `127.0.0.1:9050`; circuits rotate per request |

> [!IMPORTANT]
> Tor mode requires the Tor daemon running. Start it with `systemctl start tor` or simply `tor`.

### Rotating Proxy Pool

| Flag | Default | Description |
|---|---|---|
| `-proxy-pool` | `true` | Enable rotating free proxy pool for all traffic |
| `-proxy-pool-min <n>` | `5` | Minimum working proxies before scan starts |
| `-proxy-pool-refresh <n>` | `5` | Proxy pool refresh interval (minutes) |
| `-no-proxy-pool` | `false` | Disable default rotating proxy pool; run all traffic directly |

The proxy pool scrapes free HTTP/SOCKS5 proxies from 18 public lists, tests them concurrently with a 3s timeout, and rotates through working proxies in a thread-safe round-robin. All HTTP requests, DNS queries, and NSEC walk traffic are routed through the proxy pool. Dead proxies are automatically detected and removed on failure.

### Rate Limiting

| Flag | Default | Description |
|---|---|---|
| `-rl <n>` | `0` | Global max HTTP requests/sec per source (use per-source defaults) |
| `-rls <spec>` | `""` | Per-source overrides: `src=count/dur,src2=count/dur` (durations: `ms`, `s`, `m`, `h`, `d`) |

**Example:** `-rls "hackertarget=10/m,censys=10/m"`

> [!NOTE]
> Rate-limited responses (HTTP 429) trigger automatic exponential backoff with Retry-After header support.

### Meta Flags

| Flag | Description |
|---|---|
| `-deep` | Full deep scan: permute (level 3) + NSEC + Tor + 60s timeout — one-flag comprehensive scan |
| `-version` | Print version string and exit |
| `-h, -help` | Display help message |

### Subcommands

| Subcommand | Description |
|---|---|
| `doctor` | Run full system diagnostics — verifies every component before scanning |

---

## Passive Sources

Orion aggregates subdomain data from **38 passive intelligence sources**. Sources marked with `*` require an API key configured in the provider config file.

### Free Sources (no API key required)

| # | Source | Type | Description |
|---|---|---|---|
| 1 | `crt.sh` | Certificate Transparency | Public CT log search |
| 2 | `wayback` | Web Archives | Wayback Machine CDX index |
| 3 | `urlscan` | URL Scanner | URLScan.io public data |
| 4 | `anubis` | DNS Enumeration | Anubis database |
| 5 | `commoncrawl` | Web Crawls | CommonCrawl index |
| 6 | `bing` | Search Engine | Bing search (`site:*.domain`) |
| 7 | `baidu` | Search Engine | Baidu search (`site:domain`) |
| 8 | `google` | Search Engine | Google search results |
| 9 | `threatminer` | Threat Intel | ThreatMiner DNS data |
| 10 | `shodanct` | Certificate Search | Shodan certificate search |
| 11 | `rapiddns` | DNS Database | RapidDNS subdomain database |
| 12 | `thc` | DNS Enumeration | THC (The Hacker Choice) |
| 13 | `hudsonrock` | Threat Intel | HudsonRock credential/domain data |
| 14 | `submd` | DNS Enumeration | Subdomain database |
| 15 | `reconeer` | OSINT | Reconeer subdomain data |
| 16 | `robtex` | DNS Database | Robtex DNS lookup |
| 17 | `hackertarget` | DNS Enumeration | HackerTarget API (50/day free) |

### Keyed Sources (API key required)

| # | Source | API Key Variable | Description |
|---|---|---|---|
| 18 | `alienvault` | `ALIENVAULT_API_KEY` | AlienVault OTX (free account) |
| 19 | `bevigil` | `BEVIGIL_API_KEY` | BeVigil |
| 20 | `bufferover` | `BUFFEROVER_API_KEY` | BufferOver TLS data |
| 21 | `builtwith` | `BUILTWITH_API_KEY` | BuiltWith technology lookup |
| 22 | `censys` | `CENSYS_API_KEY` | Censys search engine |
| 23 | `certspotter` | `CERTSPOTTER_API_KEY` | CertSpotter CT logs (free account) |
| 24 | `chaos` | `CHAOS_API_KEY` | ProjectDiscovery Chaos |
| 25 | `digitalyama` | `DIGITALYAMA_API_KEY` | DigitalYama |
| 26 | `dnsdumpster` | `DNSDUMPSTER_API_KEY` | DNSDumpster |
| 27 | `fullhunt` | `FULLHUNT_API_KEY` | FullHunt attack surface |
| 28 | `github` | `GITHUB_API_KEY` | GitHub code search (classic token, no scopes needed) |
| 29 | `intelx` | `INTELX_API_KEY` | IntelX (format: `host:key`) |
| 30 | `leakix` | `LEAKIX_API_KEY` | LeakIX |
| 31 | `merklemap` | `MERKLEMAP_API_KEY` | MerkleMap |
| 32 | `netlas` | `NETLAS_API_KEY` | Netlas |
| 33 | `rsecloud` | `RSECLOUD_API_KEY` | RSECloud |
| 34 | `threatbook` | `THREATBOOK_API_KEY` | ThreatBook (Chinese platform) |
| 35 | `virustotal` | `VIRUSTOTAL_API_KEY` | VirusTotal domain/subdomain lookup |
| 36 | `whoisxmlapi` | `WHOISXML_API_KEY` | WhoisXML API |
| 37 | `windvane` | `WINDVANE_API_KEY` | WindVane (100 free queries) |
| 38 | `zoomeye` | `ZOOMEYE_API_KEY` | ZoomEye AI (format: `key` or `host:key`) |

---

## Active Techniques

### DNS Brute-force

Orion tests every word in its built-in wordlist (~5000 common subdomain names) against the target domain. It supports:

- **Configurable concurrency** via `-t` (default: `NumCPU × 10`)
- **Rate-limited DNS lookups** via `-dns-rate` (token-bucket per second)
- **Automatic retries** with exponential backoff + jitter via `-dns-retries`
- **Smart stop conditions** — aborts early when:
  - Error rate exceeds 50% in last 100 samples
  - Spurious errors exceed threshold
  - Timeout rate exceeds 50%
- **Wildcard filtering** — detected wildcard IPs are filtered from brute-force results
- **Custom wordlists** via `-w <file>` and `-max-words <n>`
- **Real-time progress tracking** — shows completion percentage, found count, errors, and timeouts

### Permutation Engine

The permutation engine **learns naming patterns** from discovered subdomains and generates plausible variations at three configurable levels:

| Level | Name | Description | Max Candidates |
|---|---|---|---|
| 1 | Basic | Prefix/suffix variations (e.g., `dev-api`, `api-dev`, `api-backup`) | 10,000 |
| 2 | Aggressive | Number combinations (e.g., `api01`, `api-01`, `web02`, `web-02`) | 50,000 |
| 3 | Extreme | Region + sub-sub combinations (AWS regions, prefix-suffix combos) | 200,000 |

All generated candidates are resolved via DNS. Only valid (resolving) subdomains are included in results.

### NSEC Zone Walking

NSEC zone walking exploits DNSSEC-enabled domains to enumerate all existing subdomains by following the **NSEC record chain**. This works because DNSSEC requires authoritative nameservers to prove non-existence of domains via NSEC records, which inadvertently reveal the next valid domain name.

- Queries the **target's authoritative DNS** directly (your IP is visible without a proxy)
- Uses **TCP DNS** (not UDP) for reliable NSEC record retrieval
- Supports routing through **SOCKS5 proxy** or **proxy pool** for anonymity
- Limited to 50,000 iterations to prevent infinite loops
- Automatic detection of NSEC3 (responds with error: "no NSEC records — domain likely uses NSEC3")

> [!CAUTION]
> NSEC walking queries the target's authoritative nameservers directly. Without a proxy (`-proxy` or `-tor`), **your source IP is visible** to the target.

---

## Doctor Diagnostics

The `doctor` subcommand performs comprehensive system verification across 15+ check categories. Run it before every scan to confirm your environment is ready.

```
orion doctor [FLAGS]
```

### Doctor Flags

| Flag | Description |
|---|---|
| `-h, -help` | Show doctor help |
| `-json` | Output results as structured JSON for CI pipelines |
| `-v` | Print detailed per-check results alongside the summary |

### Global Flags (accepted by doctor)

| Flag | Description |
|---|---|
| `-proxy <url>` | SOCKS5 proxy URL for network tests |
| `-tor` | Test Tor connectivity |
| `-silent` | Suppress the banner header |

### Checks Performed

**System & Environment:**

- Platform (`GOOS`/`GOARCH`, CPU cores)
- Go runtime (version, goroutines, heap usage)
- Go version (minimum recommended: go1.21)
- `go.mod` validity
- Dependencies (`mod verify`)
- Build (compilation check)

**Permissions:**

- Working directory writability
- Temp directory writability
- File handle limits

**External Tools:**

- gofumpt, staticcheck, gosec, golangci-lint, shadow

**Passive Sources:** All 38 sources tested for reachability and response

**DoH Resolvers:**

- Cloudflare DNS
- Google DNS

**Network:**

- Internet connectivity (via DoH)
- System DNS resolution
- IPv4 reachability
- Tor daemon detection and version
- HTTP proxy environment variables

**DNS Benchmark:**

- System resolver (5 queries, avg/min/max)
- DoH (Cloudflare, 3 queries)

**Proxy Pool:**

- All 18 proxy sources tested individually
- Total candidates scraped
- Working proxies (HTTP + SOCKS5 count)

**Configuration Validation:**

- Conflicting flags detection
- Resolver conflicts
- Concurrency settings
- Wordlist limits
- Permutation engine configuration
- NSEC walk warnings

### Doctor Exit Codes

| Code | Meaning |
|---|---|
| 0 | All checks passed — system is ready |
| 1 | Warnings (non-critical: rate limits, optional tools missing) |
| 2 | Failures — one or more components broken or unreachable |

> [!TIP]
> Use `orion doctor -json` to integrate system readiness checks into CI/CD pipelines.

---

## Output Formats

### Plain Text (default)

```
subdomain1.example.com
subdomain2.example.com
```

### NDJSON (`-json`)

Each line is a JSON object:

```json
{"host":"subdomain1.example.com","input":"example.com","source":"crt.sh"}
{"host":"subdomain2.example.com","input":"example.com","source":"alienvault"}
```

### Host-IP CSV (`-ip`)

```
subdomain1.example.com,192.0.2.1,crt.sh
subdomain1.example.com,2606:2800:220:1:248:1893:25c8:1946,crt.sh
```

### Source-Annotated (`-cs`)

```
subdomain1.example.com,[crt.sh,alienvault]
subdomain2.example.com,[virustotal]
```

---

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | All checks passed — scan completed without errors |
| 1 | Warnings present — non-critical issues (rate limits, slow sources) |
| 2 | Fatal error — bad flags, runtime failure, or network unreachable |

---

## Architecture

### Proxy Pool Architecture

```
                   ┌─────────────────────┐
                   │  18 Public Sources  │
                   │  (GitHub raw lists) │
                   └─────────┬───────────┘
                             │
                             ▼
                   ┌─────────────────────┐
                   │   Proxy Scraper     │
                   │  (concurrent fetch) │
                   └─────────┬───────────┘
                             │
                             ▼
                   ┌─────────────────────┐
                   │   Proxy Tester      │
                   │  (3s timeout, HTTP  │
                   │   HEAD + SOCKS5)    │
                   └─────────┬───────────┘
                             │
                             ▼
                   ┌─────────────────────┐
                   │   Proxy Pool        │
                   │  (thread-safe,      │
                   │   round-robin)      │
                   └─────────┬───────────┘
                             │
             ┌───────────────┼───────────────┐
             ▼               ▼               ▼
      ┌──────────┐   ┌──────────┐   ┌──────────┐
      │  HTTP    │   │  SOCKS5  │   │  DNS     │
      │ Requests │   │  Proxies │   │ Queries  │
      └──────────┘   └──────────┘   └──────────┘
```

- **18 sources** scraped concurrently
- **All proxies tested** with 3s timeout via HTTP HEAD + SOCKS5 dial
- **Round-robin rotation** across working proxies
- **Dead proxy detection** — proxies failing 2+ requests are removed
- **Automatic refresh** every N minutes (configurable via `-proxy-pool-refresh`)
- **Supports SOCKS5 for DNS** — NSEC walk and DNS queries can route through SOCKS5 proxies
- **Circuit breaker** falls back to direct connection if no proxy succeeds

### Scan Pipeline

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Parse CLI   │ →  │  Load Config │ →  │  Init Runner │ →  │  Doctor      │
│  Flags       │    │  + API Keys  │    │  + Sources   │    │  (optional)  │
└──────────────┘    └──────────────┘    └──────────────┘    └──────┬───────┘
                                                                   ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  Write       │ ←  │  Active      │ ←  │  Passive     │ ←  │  Init Proxy  │
│  Output      │    │  Brute-force │    │  Gathering   │    │  Pool        │
│  (all fmts)  │    │  + Permute   │    │  (38 sources)│    │  (18 lists)  │
│              │    │  + NSEC Walk │    │              │    │              │
└──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘
                            │
                            ▼
                   ┌──────────────────┐
                   │  Wildcard Filter │
                   │  + De-dup        │
                   └──────────────────┘
```

---

## DNS-over-HTTPS

Orion uses **10 DoH providers** in round-robin for DNS resolution:

| # | Provider | URL |
|---|---|---|
| 1 | **Cloudflare** | `https://cloudflare-dns.com/dns-query` |
| 2 | **Google** | `https://dns.google/dns-query` |
| 3 | **Quad9** | `https://dns.quad9.net/dns-query` |
| 4 | **OpenDNS** | `https://doh.opendns.com/dns-query` |
| 5 | **CleanBrowsing** | `https://doh.cleanbrowsing.org/doh/dns-query` |
| 6 | **AdGuard** | `https://dns.adguard-dns.com/dns-query` |
| 7 | **Mullvad** | `https://adblock.doh.mullvad.net/dns-query` |
| 8 | **ControlD** | `https://freedns.controld.com/p1` |
| 9 | **NextDNS** | `https://dns.nextdns.io/dns-query` |
| 10 | **DNSForge** | `https://dnsforge.de/dns-query` |

Each provider failure triggers automatic fallback to the next provider. If all DoH providers fail, fall back to UDP DNS by setting `-doh=false`.

> [!NOTE]
> DoH is enabled by default (`-doh=true`). Disable it with `-doh=false` to use traditional UDP resolvers instead.

---

## Configuration

### Provider Config

API keys are read from `~/.config/orion/provider-config.yaml`. A default config is automatically created on first run:

```yaml
# orion provider config
# Fill in your API keys for the sources you want to use.

# --- Free sources (no key needed) ---
# crtsh, wayback, urlscan, anubis, commoncrawl, bing, baidu, google,
# threatminer, shodanct, rapiddns, thc, hudsonrock, submd, reconeer,
# robtex, hackertarget

# --- Sources requiring API keys ---
# ALIENVAULT_API_KEY: https://otx.alienvault.com (free account)
ALIENVAULT_API_KEY: []
# VIRUSTOTAL_API_KEY: https://virustotal.com
VIRUSTOTAL_API_KEY: ["your-api-key-here"]
```

### Environment Variables

Environment variables override config file values:

| Variable | Description |
|---|---|
| `VIRUSTOTAL_API_KEY` | VirusTotal API key |
| `CERTSPOTTER_API_KEY` | CertSpotter API key |
| `GITHUB_TOKEN` | GitHub API key (higher rate limits) |
| `HTTP_PROXY` / `HTTPS_PROXY` | System HTTP proxy (detected by doctor) |

> [!TIP]
> Use environment variables when running in CI/CD or containerized environments where config files are impractical.

---

## Performance & Tuning

### Threading Model

Orion uses Go goroutines for concurrency. The default thread count is `NumCPU × 10`, which balances CPU and I/O-bound work well for most systems.

| Scenario | Recommended `-t` |
|---|---|
| Desktop (4-8 cores) | 40-80 |
| Server (16-32 cores) | 160-320 |
| High-bandwidth / production | 500-1000 |
| Low-bandwidth / VPN | 10-20 |

### DNS Optimization

- **DoH is faster than UDP** in most environments — it bypasses ISP DNS interceptors and benefits from HTTP/2 multiplexing
- **`-dns-rate` auto-tuning**: set to `0` to let Orion auto-calibrate; set to a fixed value (e.g., `1000`) for precise control
- **`-dns-retries`**: retransmission at 2s intervals with exponential jitter. Higher values protect against packet loss but increase scan time

### Memory & Bandwidth

| Setting | Effect |
|---|---|
| `-max-words 50000` | Moderate memory (~50MB for DNS responses) |
| `-max-words 500000` | Heavy memory (~500MB+), large wordlists |
| `-timeout 120` | Longer per-source wait, more results from slow sources |
| `-proxy-pool-refresh 10` | Reduce proxy pool scraping frequency |

### Quick Performance Profile

```sh
# Fast scan (~30s, ~5000 DNS queries)
orion -d example.com

# Deep scan (~5min, ~50k permutations + NSEC)
orion -d example.com -deep

# Maximum coverage (~30min+)
orion -d example.com -w huge.txt -max-words 500000 -permute -permute-level 3 -nsec -tor
```

---

## Security & Ethics

### Responsible Use

Orion is a reconnaissance tool designed for **authorized security assessments only**. Use it responsibly:

- **Only scan domains you own** or have explicit written permission to test
- **Respect rate limits** — Orion's built-in rate limiting helps, but configure `-rl` and `-rls` to avoid overwhelming target infrastructure
- **HackerTarget** free tier limits to 50 queries/day — be mindful of shared quotas
- **NSEC walking** can generate significant DNS traffic — use `-proxy` or `-tor` to protect your identity

### Data Privacy

- **Passive sources** send domain names to third-party APIs (VirusTotal, AlienVault, etc.) — consider the privacy implications of sharing your targets
- **Tor mode** (`-tor`) routes all traffic through the Tor network, preventing passive sources from seeing your real IP
- **The built-in proxy pool** routes traffic through free public proxies — avoid sending sensitive data

### Legal Disclaimer

> [!CAUTION]
> Unauthorized scanning or enumeration of systems you do not own or have permission to test may violate:
> - Computer Fraud and Abuse Act (CFAA) — USA
> - Computer Misuse Act — UK
> - Similar laws in other jurisdictions
>
> **You are solely responsible for complying with all applicable laws.**

---

## FAQ & Troubleshooting

### General

**Q: How is Orion different from subfinder, amass, or other tools?**

Orion combines the breadth of passive sources (38) with active techniques (brute-force, permutation engine, NSEC walk) and built-in infrastructure (proxy pool, DoH, Tor) in a single binary. It also includes the doctor diagnostic system for proactive issue detection.

**Q: Why does my scan return fewer results than expected?**

1. Run `orion doctor` to verify passive sources are reachable
1. Some sources have daily rate limits (HackerTarget: 50/day)
1. Configure API keys for keyed sources to unlock full potential
1. The target domain may have minimal subdomain footprint

**Q: How do I get API keys?**

Most keyed sources offer free tiers:

- [VirusTotal](https://virustotal.com) — free account, ~500 requests/day
- [AlienVault OTX](https://otx.alienvault.com) — free account
- [CertSpotter](https://certspotter.com) — free account
- [Chaos](https://chaos.projectdiscovery.io) — free API key
- [GitHub](https://github.com/settings/tokens) — classic token, no scopes needed

### Proxy & Connectivity

**Q: The proxy pool isn't finding any working proxies.**

1. Run `orion doctor` to check proxy source reachability
2. Try lowering `-proxy-pool-min` (e.g., `-proxy-pool-min 1`)
3. Free proxies are unreliable by nature — consider using `-tor` for reliable anonymity
4. Increase `-proxy-pool-refresh` to give the pool more scraping time

**Q: Tor mode isn't working.**

1. Verify Tor is running: `systemctl status tor` or `ps aux | grep tor`
2. Ensure Tor is listening on `127.0.0.1:9050`
3. Run `orion doctor -tor` for detailed Tor diagnostics
4. Check firewall rules — Tor uses port 9050 for SOCKS

### Performance

**Q: The scan is too slow.**

1. Increase `-t` for more concurrent workers
2. Set `-dns-rate` to `0` for auto-tuning
3. Limit passive sources: `-sources crtsh,alienvault,virustotal`
4. Skip NSEC walk (`-nsec` off by default)
5. Use a smaller wordlist or `-max-words`

**Q: Orion is using too much memory.**

1. Reduce `-max-words` for smaller wordlists
2. Reduce `-t` to limit concurrent goroutines
3. Disable permutation engine (`-permute` off by default)
4. Use `-max-words 5000` with the built-in wordlist

### Errors

#### Q: "no input provided"

Use `-d <domain>` or `-dL <file>` to specify targets. You can also pipe domains via stdin.

#### Q: "proxy circuit breaker: traffic blocked"

All proxy options (pool, Tor, SOCKS5) failed. Check your proxy configuration or disable with `-no-proxy-pool` and `-proxy ""`.

#### Q: "no NSEC records — domain likely uses NSEC3"

The target uses NSEC3 (NSEC with cryptography), which Orion cannot enumerate. This is expected for DNSKEY-configured domains with NSEC3.

---

## Build System

### Makefile Targets

| Target | Description |
|---|---|
| `help` | Show available targets and variables |
| `all` | Full CI pipeline: lint-fast → build → install |
| `build` | Build binary for current platform |
| `install` | Build and install to `$GOBIN` or `$GOPATH/bin` |
| `run` | Build and run (pass `ARGS=...` for flags) |
| `build-linux` | Build all Linux targets (amd64, 386, arm64, armv6, armv7) |
| `build-linux-rpm` | Alias for build-linux (RPM packaging) |
| `build-linux-deb` | Alias for build-linux (DEB packaging) |
| `build-linux-arch` | Alias for build-linux (Arch Linux packaging) |
| `build-linux-alpine` | Build for Alpine Linux (musl, CGO_ENABLED=0) |
| `build-termux` | Build for Termux / Android (CGo-free) |
| `build-termux-cgo` | Build for Termux / Android (requires Android NDK) |
| `build-darwin` | Build all macOS targets (amd64, arm64) |
| `build-windows` | Build all Windows targets (amd64, 386) |
| `build-all` | Build for all supported platforms (CGo-free) |
| `release` | Create a GitHub release (`BUMP=patch\|minor\|major\|vX.Y.Z`) |
| `test` | Run all tests |
| `test-race` | Run all tests with the race detector |
| `test-short` | Run all tests in short mode (skips expensive ones) |
| `test-cover` | Run all tests with code coverage |
| `lint` | Run full lint suite (format, vet, race, static analysis) |
| `lint-fast` | Run lint in short mode (skips gosec) |
| `fmt` | Format all Go code and tidy module |
| `vet` | Run `go vet` |
| `clean` | Remove all build artifacts |

### Build Variables

| Variable | Default | Description |
|---|---|---|
| `GO` | `go` | Go compiler binary |
| `BUILDDIR` | `build` | Artifact output directory |
| `GOFLAGS` | `-trimpath` | Extra build flags |
| `LDFLAGS` | `-s -w` | Linker flags |
| `Q` | `@` | Set to empty for verbose output |
| `PREFIX` | `/usr/local` | Install prefix |
| `DESTDIR` | `""` | Install destination directory |

### Release Process

```sh
# Dry-run (preview everything without publishing)
make release BUMP=patch   # or minor, major, vX.Y.Z

# Actual release
make release BUMP=minor
```

The release script (`scripts/release.sh`) handles:

1. **Semver bump** (patch/minor/major) or explicit version
2. **Dirty-tree guard** and untracked file warnings
3. **Linting** via `make lint-fast`
4. **Cross-platform build** via `make build-all`
5. **Binary compression** (`.tar.gz` for Linux/macOS, `.zip` for Windows)
6. **SHA256 checksums** for all artifacts
7. **Editor review** of release notes before publishing
8. **Annotated tag** creation
9. **GitHub release** with all artifacts uploaded

---

## Development

### Prerequisites

| Requirement | Version |
|---|---|
| Go | 1.21+ (1.25+ recommended) |
| `make` | Any version |
| gofumpt | Optional (linting) |
| staticcheck | Optional (linting) |
| gosec | Optional (linting) |
| golangci-lint | Optional (linting) |
| shadow | Optional (linting) |

### Linting

```sh
# Full lint suite (gofumpt, go fmt, go vet, staticcheck, gosec,
# golangci-lint, shadow, deadcode)
make lint

# Fast lint (skips gosec)
make lint-fast
```

The lint script (`scripts/lint.sh`) runs these checks in parallel:

| Check | Purpose |
|---|---|
| `gofumpt` | Strict formatting |
| `go fmt` | Standard formatting |
| `go fix` | Old API pattern detection |
| `go mod tidy` | Module cleanliness |
| `go test -race` | Race detector |
| `go vet` | Suspicious constructs |
| `staticcheck` | Static analysis |
| `gosec` | Security inspection |
| `golangci-lint` | 100+ linters (`.golangci.yml`) |
| `shadow` | Variable shadowing detection |
| `deadcode` | Unreachable function detection |
| `go mod verify` | Dependency integrity |

### Testing

```sh
# All tests
make test

# With race detector
make test-race

# Short mode (skips network-dependent tests)
make test-short

# With coverage
make test-cover
```

### Version Management

Version is managed from a **single source of truth** — `metadata.yaml`:

```yaml
project_name: orion
version: 0.1.0
```

The version is injected at build time via ldflags (`-X main.version=$(VERSION)`). The `version` variable in `main.go` is always `"dev"` — the real version comes from the Makefile, which reads `metadata.yaml`. Tests enforce that no source file hardcodes the version string.

---

## Examples

### Basic Enumeration

```sh
# Single target, default settings
orion -d example.com

# Save results to file
orion -d example.com -o results.txt

# Batch mode with domain list
orion -dL domains.txt

# Batch mode, all results to one file
orion -dL targets.txt -o all-results.txt
```

### Deep Mode

```sh
# One-flag comprehensive scan: permute (level 3) + NSEC + Tor + 60s timeout
orion -d example.com -deep

# Deep mode with file output
orion -d example.com -deep -o results.txt

# Batch deep scan (silent)
orion -dL domains.txt -deep -silent
```

### Passive Only

```sh
# No contact with target DNS
orion -d example.com -passive

# Passive + permutation (no brute-force)
orion -d example.com -passive -permute

# Passive only, file output
orion -d example.com -passive -permute -o passive-only.txt
```

### Custom Wordlist Brute-force

```sh
# Use a custom wordlist
orion -d example.com -w ~/wordlists/big.txt

# Limit wordlist to first 10,000 entries
orion -d example.com -w custom.txt -max-words 10000

# High-concurrency brute-force with large wordlist
orion -d example.com -w ~/lists/all.txt -max-words 50000 -t 500
```

### Permutation Engine (Examples)

```sh
# Basic permutation (level 2 default)
orion -d example.com -permute

# Extreme permutation (level 3)
orion -d example.com -permute -permute-level 3

# Deep mode with controlled permute level
orion -d example.com -deep -permute-level 2
```

### NSEC Zone Walking (Examples)

```sh
# NSEC walk (your IP visible to target DNS)
orion -d example.com -nsec

# NSEC walk anonymized via proxy
orion -d example.com -nsec -proxy socks5://127.0.0.1:9050
```

### Tor / Anonymous Scanning

```sh
# Route everything through Tor
orion -d example.com -tor

# Custom SOCKS5 proxy
orion -d example.com -proxy socks5://127.0.0.1:9050

# Full anonymity: Tor + deep scan
orion -d example.com -tor -deep
```

### Automation & Piping

```sh
# Pipe results to another tool
orion -d example.com -silent | tee -a results.txt

# Sort unique results for piped processing
orion -d example.com -silent -passive | sort -u | httpx

# Count results
orion -d example.com -silent -o results.txt && wc -l results.txt

# JSON output with jq filtering
orion -d example.com -json | jq -r '.host' | sort -u

# CI-friendly JSON with source attribution
orion -d example.com -json -cs | jq -c 'select(.source | contains("virustotal"))'
```

### High-performance / Production

```sh
# Production-grade deep scan
orion -d example.com -deep -t 500 -timeout 120

# Bulk domain processing
orion -dL domains.txt -deep -silent -o bulk-results.txt

# Maximum coverage: huge wordlist + permute + NSEC + Tor
orion -d example.com -w huge.txt -max-words 100000 -permute -nsec -tor

# Rate-limited production scan (respectful to sources)
orion -d example.com -rl 5 -rls "hackertarget=1/m,censys=5/m" -t 200
```

### Diagnostics

```sh
# Quick system check
orion doctor

# Detailed diagnostics
orion doctor -v

# JSON output for CI pipelines
orion doctor -json

# Diagnostics through proxy
orion doctor -proxy socks5://127.0.0.1:9050
```

### Advanced Workflows

```sh
# Per-domain output files
orion -dL domains.txt -oD results/

# Selected sources only
orion -d example.com -sources crtsh,alienvault,virustotal -permute

# Exclude specific sources
orion -d example.com -es hackertarget,censys

# Match/filter patterns
orion -d example.com -match "*.api.*" -filter "*.dev.*"

# Wildcard removal + IP output
orion -d example.com -nW -ip

# Source statistics after scan
orion -d example.com -deep -stats
```

---

## Contributing

We welcome contributions! Here's how to get started:

1. **Fork** the repository
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make your changes** following our code style
4. **Run the linter**: `make lint`
5. **Run tests**: `make test`
6. **Commit** using conventional commits:
   - `feat:` — new feature
   - `fix:` — bug fix
   - `chore:` — maintenance
   - `docs:` — documentation
   - `test:` — testing
   - `refactor:` — code restructuring
7. **Push** and open a **Pull Request**

### Code Style

- Follow standard Go conventions (`gofumpt` formatting)
- Use `goimports` for import organization
- Avoid global state (exceptions are annotated with `//nolint:gochecknoglobals`)
- Add tests for new functionality
- Keep cyclomatic complexity under 20 (checked by `gocyclo`)
- Run `make lint` before submitting

### Pull Request Guidelines

- Keep PRs focused on a single change
- Write clear PR descriptions explaining the "why"
- Include before/after screenshots for UI changes
- Update documentation when adding or changing flags

---

## License

MIT License — Copyright (c) 2026 Shesher Hasan

See the [LICENSE](LICENSE) file for details.

---

<p align="center">
  <a href="https://github.com/myth-tools/orion">GitHub</a> &bull;
  <a href="https://github.com/myth-tools/orion/issues">Issues</a> &bull;
  <a href="https://github.com/myth-tools/orion/releases">Releases</a>
  <br><br>
  <sub>Built with Go — 38 passive sources · 18 proxy pool sources · 10 DoH providers</sub>
</p>
