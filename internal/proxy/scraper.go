package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	errNoProxiesScraped       = errors.New("no proxies scraped from any source")
	errUnsupportedProxyScheme = errors.New("unsupported proxy scheme")
)

const (
	ProxyTypeHTTP   = "http"
	ProxyTypeSOCKS4 = "socks4"
	ProxyTypeSOCKS5 = "socks5"
)

type Proxy struct {
	Addr     string
	Type     string
	Alive    bool
	Username string
	Password string
}

func ParseProxyURL(rawURL string) (Proxy, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Proxy{}, fmt.Errorf("invalid proxy URL %q: %w", rawURL, err)
	}
	p := Proxy{Addr: u.Host}
	switch u.Scheme {
	case "http", "https":
		p.Type = ProxyTypeHTTP
	case "socks5", "socks5h":
		p.Type = ProxyTypeSOCKS5
	default:
		return Proxy{}, fmt.Errorf("%w %q (use http, https, socks5, or socks5h)", errUnsupportedProxyScheme, u.Scheme)
	}
	if u.User != nil {
		p.Username = u.User.Username()
		p.Password, _ = u.User.Password()
	}
	return p, nil
}

type Scraper struct {
	client  *http.Client
	sources []string
}

func NewScraper(client *http.Client) *Scraper {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	client.Timeout = 10 * time.Second

	return &Scraper{
		client: client,
		sources: []string{
			"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
			"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
			"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/http.txt",
			"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/socks5.txt",
			"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-http.txt",
			"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-socks5.txt",
			"https://raw.githubusercontent.com/proxifly/free-proxy-list/main/proxies/protocols/http/data.txt",
			"https://raw.githubusercontent.com/proxifly/free-proxy-list/main/proxies/protocols/socks5/data.txt",
			"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/http.txt",
			"https://raw.githubusercontent.com/vakhov/fresh-proxy-list/master/socks5.txt",
			"https://raw.githubusercontent.com/rdavydov/proxy-list/main/proxies/http.txt",
			"https://raw.githubusercontent.com/rdavydov/proxy-list/main/proxies/socks5.txt",
			"https://raw.githubusercontent.com/mmpx12/proxy-list/master/http.txt",
			"https://raw.githubusercontent.com/mmpx12/proxy-list/master/socks5.txt",
			"https://raw.githubusercontent.com/proxygenerator1/ProxyGenerator/main/MostStable/http.txt",
			"https://raw.githubusercontent.com/proxygenerator1/ProxyGenerator/main/MostStable/socks5.txt",
			"https://raw.githubusercontent.com/Thordata/awesome-free-proxy-list/main/proxies/http.txt",
			"https://raw.githubusercontent.com/Thordata/awesome-free-proxy-list/main/proxies/socks5.txt",
		},
	}
}

func (s *Scraper) AddSource(url string) {
	s.sources = append(s.sources, url)
}

func (s *Scraper) Sources() []string {
	cp := make([]string, len(s.sources))
	copy(cp, s.sources)
	return cp
}

func TypeFromSourceURL(url string) string {
	u := strings.ToLower(url)
	switch {
	case strings.Contains(u, "socks5"):
		return ProxyTypeSOCKS5
	case strings.Contains(u, "socks4"):
		return ProxyTypeSOCKS4
	default:
		return ProxyTypeHTTP
	}
}

func (s *Scraper) Scrape(ctx context.Context) ([]Proxy, error) {
	type result struct {
		proxies []Proxy
		err     error
		url     string
	}

	ch := make(chan result, len(s.sources))
	for _, src := range s.sources {
		go func(url string) {
			proxies, err := s.fetchProxies(ctx, url)
			ch <- result{proxies: proxies, err: err, url: url}
		}(src)
	}

	seen := make(map[string]bool)
	var all []Proxy
	for range s.sources {
		select {
		case res := <-ch:
			for _, p := range res.proxies {
				key := p.Type + "://" + p.Addr
				if !seen[key] {
					seen[key] = true
					all = append(all, p)
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if len(all) == 0 {
		return nil, errNoProxiesScraped
	}

	rand.Shuffle(len(all), func(i, j int) {
		all[i], all[j] = all[j], all[i]
	})

	return all, nil
}

var proxyLineRe = regexp.MustCompile(`^([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}:[0-9]+)$`)

var schemePrefixes = []string{
	"http://", "https://", "socks4://", "socks4a://", "socks5://", "socks5h://",
}

func (s *Scraper) fetchProxies(ctx context.Context, url string) ([]Proxy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("req: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	proxyType := detectType(url)
	lines := strings.Split(string(body), "\n")
	var proxies []Proxy
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		addr := line
		for _, prefix := range schemePrefixes {
			if strings.HasPrefix(strings.ToLower(addr), prefix) {
				addr = addr[len(prefix):]
				break
			}
		}
		if proxyLineRe.MatchString(addr) {
			proxies = append(proxies, Proxy{Addr: addr, Type: proxyType})
		}
	}

	return proxies, nil
}

func detectType(url string) string {
	return TypeFromSourceURL(url)
}
