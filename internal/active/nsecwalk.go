package active

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var (
	errNoNameservers  = errors.New("no nameservers found")
	errNoNameserverIP = errors.New("could not resolve any nameserver IP")
	errNoNSECResponse = errors.New("no response from target NS")
	errNoNSECRecords  = errors.New("no NSEC records — domain likely uses NSEC3 or DNSSEC not enabled")
	errNoStartDomain  = errors.New("no starting domain from NSEC records")
)

type NSECWalker struct {
	client     *dns.Client
	dialer     *ProxyDialer
	poolDialer func(ctx context.Context, network, addr string) (net.Conn, error)
	timeout    time.Duration
}

func NewNSECWalker(dialer *ProxyDialer, poolDialer func(ctx context.Context, network, addr string) (net.Conn, error)) *NSECWalker {
	return &NSECWalker{
		client:     &dns.Client{Timeout: 10 * time.Second, Net: "tcp"},
		dialer:     dialer,
		poolDialer: poolDialer,
		timeout:    10 * time.Second,
	}
}

func (n *NSECWalker) exchange(ctx context.Context, m *dns.Msg, addr string) (*dns.Msg, error) {
	if n.poolDialer != nil {
		raw, err := n.poolDialer(ctx, "tcp", addr)
		if err == nil {
			co := &dns.Conn{Conn: raw}
			defer co.Close()
			r, _, err := n.client.ExchangeWithConn(m, co)
			return r, err
		}
	}
	if n.dialer != nil && n.dialer.Enabled() {
		raw, err := n.dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("proxy dial to %s: %w", addr, err)
		}
		co := &dns.Conn{Conn: raw}
		defer co.Close()
		r, _, err := n.client.ExchangeWithConn(m, co)
		return r, err
	}
	r, _, err := n.client.Exchange(m, addr)
	return r, err
}

func (n *NSECWalker) findNameservers(ctx context.Context, domain string) []string {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeNS)
	m.SetEdns0(4096, false)

	servers := []string{
		"1.1.1.1:53",
		"8.8.8.8:53",
	}

	for _, server := range servers {
		r, err := n.exchange(ctx, m, server)
		if err != nil || r == nil {
			continue
		}
		if r.Rcode != dns.RcodeSuccess {
			continue
		}
		var ns []string
		for _, ans := range r.Ns {
			if nsRR, ok := ans.(*dns.NS); ok {
				ns = append(ns, nsRR.Ns)
			}
		}
		if len(ns) > 0 {
			return ns
		}
	}

	return []string{domain, "ns1." + domain, "ns2." + domain}
}

func (n *NSECWalker) resolveNameserverIP(ctx context.Context, ns string) string {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(ns), dns.TypeA)

	r, err := n.exchange(ctx, m, "1.1.1.1:53")
	if err != nil || r == nil {
		return ""
	}
	for _, ans := range r.Answer {
		if aRR, ok := ans.(*dns.A); ok {
			return aRR.A.String() + ":53"
		}
	}
	return ""
}

func (n *NSECWalker) Walk(ctx context.Context, domain string, results chan<- string) error {
	domain = strings.ToLower(domain)
	nameservers := n.findNameservers(ctx, domain)
	if len(nameservers) == 0 {
		return fmt.Errorf("%w for %s", errNoNameservers, domain)
	}

	nsIP := n.resolveNameserverIP(ctx, nameservers[0])
	if nsIP == "" {
		for _, ns := range nameservers {
			nsIP = n.resolveNameserverIP(ctx, ns)
			if nsIP != "" {
				break
			}
		}
	}
	if nsIP == "" {
		return fmt.Errorf("%w for %s", errNoNameserverIP, domain)
	}

	m := new(dns.Msg)
	m.SetEdns0(4096, true)
	randStr := randomDNSProbeName(domain)
	m.SetQuestion(dns.Fqdn(randStr), dns.TypeA)

	r, err := n.exchange(ctx, m, nsIP)
	if err != nil || r == nil {
		return fmt.Errorf("%w %s", errNoNSECResponse, nsIP)
	}

	var nsecRecords []*dns.NSEC
	for _, rr := range r.Ns {
		if nsec, ok := rr.(*dns.NSEC); ok {
			nsecRecords = append(nsecRecords, nsec)
		}
	}

	if len(nsecRecords) == 0 {
		return errNoNSECRecords
	}

	return n.walkChain(ctx, domain, nsIP, nsecRecords, results)
}

func randomDNSProbeName(domain string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return fmt.Sprintf("nsec-probe-%d.%s", time.Now().UnixNano(), domain)
	}
	return fmt.Sprintf("nsec-probe-%d.%s", n.Int64(), domain)
}

func (n *NSECWalker) walkChain(ctx context.Context, domain, nsIP string, nsecRecords []*dns.NSEC, results chan<- string) error {
	domain = strings.ToLower(domain)
	seen := make(map[string]bool)
	visited := make(map[string]bool)

	if len(nsecRecords) == 0 || nsecRecords[0] == nil {
		return errNoStartDomain
	}

	currentDomain := nsecRecords[0].Header().Name
	if currentDomain == "" {
		return errNoStartDomain
	}

	const maxIterations = 50000

	for range maxIterations {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if visited[currentDomain] {
			break
		}
		visited[currentDomain] = true

		query := new(dns.Msg)
		query.SetEdns0(4096, true)
		query.SetQuestion(dns.Fqdn(currentDomain), dns.TypeA)

		resp, err := n.exchange(ctx, query, nsIP)
		if err != nil {
			return err
		}
		if resp == nil {
			break
		}

		var foundNSEC *dns.NSEC
		for _, rr := range resp.Ns {
			if nsec, ok := rr.(*dns.NSEC); ok {
				foundNSEC = nsec
				break
			}
		}

		if foundNSEC == nil {
			break
		}

		currentOwner := strings.TrimSuffix(foundNSEC.Header().Name, "."+domain+".")

		if currentOwner != "" && !seen[currentOwner] {
			seen[currentOwner] = true
			sub := currentOwner + "." + domain
			select {
			case results <- sub:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if foundNSEC.NextDomain == dns.Fqdn(domain) {
			break
		}

		currentDomain = foundNSEC.NextDomain
	}

	return nil
}
