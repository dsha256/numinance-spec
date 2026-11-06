// Package replay serves recorded, redacted GitHub Actions exchanges to the ci-github-actions provider, so the unit tests, the conformance
// kit and the host-level acceptance need no network and no token (ADR 0015, replay mode). A cassette is a list of HTTP
// exchanges (method, path, request body, status, JSON answer); a request with no recorded exchange fails closed.
package replay

import (
	"embed"
	"fmt"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

//go:embed cassettes/*.json
var cassettes embed.FS

// Transport returns the cassette transport over every embedded cassette, as the provider's transport in replay mode.
func Transport() (ghprovider.Transport, error) {
	entries, err := cassettes.ReadDir("cassettes")
	if err != nil {
		return ghprovider.Transport{}, err
	}
	var all []ghprovider.Cassette
	for _, e := range entries {
		b, err := cassettes.ReadFile("cassettes/" + e.Name())
		if err != nil {
			return ghprovider.Transport{}, err
		}
		c, err := ghprovider.ParseCassette(b)
		if err != nil {
			return ghprovider.Transport{}, fmt.Errorf("%s: %w", e.Name(), err)
		}
		all = append(all, c)
	}
	rt, err := ghprovider.NewReplay(all...)
	if err != nil {
		return ghprovider.Transport{}, err
	}
	return ghprovider.Transport{RT: rt, Replay: true}, nil
}

// Fixed values of the cassette: the repository and the commit its answers name.
const (
	Repository = "acme/shop"
	Revision   = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee01"
)
