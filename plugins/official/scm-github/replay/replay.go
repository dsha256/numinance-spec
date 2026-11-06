// Package replay serves recorded, redacted GitHub exchanges to the scm-github provider, so the unit tests, the conformance
// kit and the host-level acceptance need no network and no token (ADR 0015, replay mode). A cassette is a list of HTTP
// exchanges (method, path, request body, status, JSON answer); a request with no recorded exchange fails closed.
package replay

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/numinance/numinance-spec/plugins/official/internal/ghprovider"
)

//go:embed cassettes/*.json
var cassettes embed.FS

// maxCassetteBytes bounds a cassette file read from disk.
const maxCassetteBytes = 4 << 20

// readCassetteFile reads the cassette at an absolute path, refusing one over its bound.
func readCassetteFile(path string) (ghprovider.Cassette, error) {
	if !filepath.IsAbs(path) {
		return ghprovider.Cassette{}, errors.New("a cassette path is absolute")
	}
	path = filepath.Clean(path)
	// A regular file only: opening a FIFO would block the launch, and a directory or device is no cassette. The check before the open
	// keeps the open from blocking; the one after it checks what was opened.
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return ghprovider.Cassette{}, errors.New("a cassette is a regular file")
	}
	f, err := os.Open(path) //nolint:gosec // the path is an argument of the host that launches this test build
	if err != nil {
		return ghprovider.Cassette{}, err
	}
	defer func() { _ = f.Close() }()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return ghprovider.Cassette{}, errors.New("a cassette is a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxCassetteBytes+1))
	if err != nil {
		return ghprovider.Cassette{}, err
	}
	if len(b) > maxCassetteBytes {
		return ghprovider.Cassette{}, errors.New("a cassette is over its size bound")
	}
	c, err := ghprovider.ParseCassette(b)
	if err != nil {
		return ghprovider.Cassette{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Transport returns the cassette transport over every embedded cassette and the cassette files at the given absolute paths (a
// scenario of one run, see PullRequestCassette), as the provider's transport in replay mode. Matching stays exact for all of them.
func Transport(files ...string) (ghprovider.Transport, error) {
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
	for _, f := range files {
		c, err := readCassetteFile(f)
		if err != nil {
			return ghprovider.Transport{}, err
		}
		all = append(all, c)
	}
	rt, err := ghprovider.NewReplay(all...)
	if err != nil {
		return ghprovider.Transport{}, err
	}
	return ghprovider.Transport{RT: rt, Replay: true}, nil
}

// Fixed values of the cassette: the repository and the commits its answers name.
const (
	Repository   = "acme/shop"
	BaseRevision = "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"
	Candidate    = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee01"
	MergeCommit  = "feedface00000000feedface00000000feedface"
	MovedHead    = "bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0"
)
