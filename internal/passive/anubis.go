package passive

import (
	"context"
	"fmt"
)

type Anubis struct{}

func (a *Anubis) Name() string       { return "anubis" }
func (a *Anubis) NeedsKey() bool     { return false }
func (a *Anubis) SetKeys(_ []string) {}

func (a *Anubis) Fetch(ctx context.Context, domain string, results chan<- string) error {
	return fetchSliceSource(ctx, "anubis", fmt.Sprintf("https://anubisdb.com/subdomains/%s", domain), results)
}
