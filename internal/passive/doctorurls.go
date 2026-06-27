package passive

import (
	"fmt"
)

type SourceTestURL struct {
	Name string
	URL  func(domain string) string
}

func SourceTestURLs() []SourceTestURL {
	all := AllSources(nil)
	urls := make([]SourceTestURL, 0, len(all))
	for _, s := range all {
		if tu, ok := s.(interface{ TestURL(url string) string }); ok {
			urls = append(urls, SourceTestURL{Name: s.Name(), URL: tu.TestURL})
		}
	}
	return urls
}

// TestURL returns a health-check URL for each passive source.
func (c *CRTSH) TestURL(domain string) string {
	return fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain)
}

func (a *AlienVault) TestURL(domain string) string {
	return fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/passive_dns", domain)
}

func (w *Wayback) TestURL(domain string) string {
	return fmt.Sprintf("https://archive.org/wayback/available?url=%s", domain)
}

func (u *URLScan) TestURL(domain string) string {
	return fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=10000", domain)
}

func (a *Anubis) TestURL(domain string) string {
	return fmt.Sprintf("https://anubisdb.com/subdomains/%s", domain)
}

func (c *CertSpotter) TestURL(domain string) string {
	return fmt.Sprintf("https://api.certspotter.com/v1/issuances?domain=%s&include_subdomains=true&expand=dns_names", domain)
}

func (h *Hackertarget) TestURL(domain string) string {
	return fmt.Sprintf("https://api.hackertarget.com/hostsearch/?q=%s", domain)
}

func (c *CommonCrawl) TestURL(domain string) string {
	return fmt.Sprintf("https://index.commoncrawl.org/CC-MAIN-test-index?url=*.%s&output=json", domain)
}

func (b *Bing) TestURL(domain string) string {
	return fmt.Sprintf("https://www.bing.com/search?q=site:*.%s&count=50", domain)
}

func (b *Baidu) TestURL(domain string) string {
	return fmt.Sprintf("https://www.baidu.com/s?wd=site:%s&rn=50", domain)
}

func (t *ThreatMiner) TestURL(domain string) string {
	return fmt.Sprintf("https://api.threatminer.org/v2/domain.php?q=%s&rt=5", domain)
}

func (s *ShodanCT) TestURL(domain string) string {
	return fmt.Sprintf("https://ctl.shodan.io/api/v1/domain/%s/hostnames", domain)
}

func (r *RapidDNS) TestURL(domain string) string {
	return fmt.Sprintf("https://rapiddns.io/subdomain/%s?page=1&full=1", domain)
}

func (h *HudsonRock) TestURL(domain string) string {
	return fmt.Sprintf("https://cavalier.hudsonrock.com/api/json/v2/osint-tools/urls-by-domain?domain=%s", domain)
}

func (v *VirusTotal) TestURL(domain string) string {
	return fmt.Sprintf("https://www.virustotal.com/api/v3/domains/%s/subdomains?limit=40", domain)
}

func (f *FullHunt) TestURL(domain string) string {
	return fmt.Sprintf("https://fullhunt.io/api/v1/domain/%s/subdomains", domain)
}

func (b *BufferOver) TestURL(domain string) string {
	return fmt.Sprintf("https://tls.bufferover.run/dns?q=.%s", domain)
}

func (l *LeakIX) TestURL(domain string) string {
	return fmt.Sprintf("https://leakix.net/api/subdomains/%s", domain)
}

func (d *DNSDumpster) TestURL(domain string) string {
	return fmt.Sprintf("https://api.dnsdumpster.com/domain/%s", domain)
}

func (w *WhoisXMLAPI) TestURL(domain string) string {
	return fmt.Sprintf("https://subdomains.whoisxmlapi.com/api/v1?apiKey=test&domainName=%s", domain)
}

func (i *IntelX) TestURL(domain string) string {
	return fmt.Sprintf("https://2.intelx.io/intelligent/search/result?id=test&limit=1&domain=%s", domain)
}

func (c *Censys) TestURL(domain string) string {
	return fmt.Sprintf("https://api.platform.censys.io/v3/global/search/query?domain=%s", domain)
}

func (g *GitHub) TestURL(domain string) string {
	return fmt.Sprintf("https://api.github.com/search/code?q=%s&per_page=1", domain)
}

func (g *Google) TestURL(domain string) string {
	return fmt.Sprintf("https://www.google.com/search?q=site:*.%s", domain)
}

func (s *Submd) TestURL(domain string) string {
	return fmt.Sprintf("https://api.sub.md/v1/search?apex=%s", domain)
}

func (r *Reconeer) TestURL(domain string) string {
	return fmt.Sprintf("https://www.reconeer.com/api/domain/%s", domain)
}

func (b *Bevigil) TestURL(domain string) string {
	return fmt.Sprintf("https://osint.bevigil.com/api/%s/subdomains/", domain)
}

func (b *BuiltWith) TestURL(domain string) string {
	return fmt.Sprintf("https://api.builtwith.com/v21/api.json?KEY=test&LOOKUP=%s", domain)
}

func (c *Chaos) TestURL(domain string) string {
	return fmt.Sprintf("https://dns.projectdiscovery.io/dns/%s/subdomains", domain)
}

func (m *MerkleMap) TestURL(domain string) string {
	return fmt.Sprintf("https://api.merklemap.com/v1/search?query=*.%s", domain)
}

func (n *Netlas) TestURL(domain string) string {
	return fmt.Sprintf("https://app.netlas.io/api/domains_count/?q=domain:%s", domain)
}

func (r *Robtex) TestURL(domain string) string {
	return fmt.Sprintf("https://freeapi.robtex.com/pdns/forward/%s", domain)
}

func (r *RSECloud) TestURL(domain string) string {
	return fmt.Sprintf("https://api.rsecloud.com/api/v2/subdomains/passive/%s?page=1", domain)
}

func (t *ThreatBook) TestURL(domain string) string {
	return fmt.Sprintf("https://api.threatbook.cn/v3/domain/sub_domains?apikey=test&resource=%s", domain)
}

func (w *WindVane) TestURL(domain string) string {
	return fmt.Sprintf("https://windvane.lichoin.com/trpc.backendhub.public.WindvaneService/ListSubDomain?domain=%s", domain)
}

func (z *ZoomEyeAPI) TestURL(domain string) string {
	return fmt.Sprintf("https://api.zoomeye.org/domain/search?q=%s&type=1", domain)
}

func (d *DigitalYama) TestURL(domain string) string {
	return fmt.Sprintf("https://api.digitalyama.com/subdomain_finder?domain=%s", domain)
}
