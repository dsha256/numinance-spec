// Package replay serves recorded, redacted tool output to the verifier-python provider, so the seeded-defect tests and the
// conformance kit need no network and no installed Python tools (ADR 0015, replay mode). A recording is the raw output of one
// tool on one candidate of testdata/candidates, written by testdata/record.sh. The recording is keyed by the digest of the
// candidate artifact the host verified and by the check; a candidate with no recording is UNAVAILABLE, never a pass.
package replay

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/numinance/numinance-spec/plugins/official/verifier-python/provider"
	typesv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/types/v1"
)

//go:embed recordings
var recordings embed.FS

// Runner is a provider.Runner over the embedded recordings.
type Runner struct{}

var _ provider.Runner = Runner{}

type meta struct {
	Check      string               `json:"check"`
	Candidate  string               `json:"candidate"`
	ExitCode   int                  `json:"exit_code"`
	Directives []provider.Directive `json:"directives"`
}

// Run implements provider.Runner. The recorded output is the tool's own; the provider parses it as it would the live one.
func (Runner) Run(_ context.Context, c provider.Call) (provider.Output, error) {
	sum, ok := strings.CutPrefix(c.CandidateDigest, "sha256:")
	if !ok || len(sum) != 64 {
		return provider.Output{}, fmt.Errorf("%w: not a candidate digest", provider.ErrUnavailable)
	}
	dir := "recordings/" + sum + "/" + c.Config.Check
	raw, err := recordings.ReadFile(dir + ".json")
	if err != nil {
		return provider.Output{}, fmt.Errorf("%w: no recording of this check for this candidate", provider.ErrUnavailable)
	}
	var m meta
	if err := json.Unmarshal(raw, &m); err != nil || m.Check != c.Config.Check {
		return provider.Output{}, fmt.Errorf("%w: the recording is damaged", provider.ErrUnavailable)
	}
	out, err := recordings.ReadFile(dir + ".out")
	if err != nil {
		return provider.Output{}, fmt.Errorf("%w: the recording has no output", provider.ErrUnavailable)
	}
	return provider.Output{Stdout: out, ExitCode: m.ExitCode, Directives: m.Directives, Replayed: true}, nil
}

// Candidate names of the recordings, the labels testdata/record.sh hashes.
const (
	Clean      = "clean"
	Seeded     = "seeded"
	Suppressed = "suppressed"
	Crashing   = "crashing"
	Malformed  = "malformed"
)

// CandidateRef is the reference of the named candidate: its digest is the one a recording is keyed by (the sha256 of
// "kit-candidate:" and the name).
func CandidateRef(name string) *typesv1.Reference {
	sum := sha256.Sum256([]byte("kit-candidate:" + name))
	return &typesv1.Reference{ArtifactRef: "candidate/" + name, Sha256: &typesv1.Digest{Value: "sha256:" + hex.EncodeToString(sum[:])}, ArtifactType: "candidate"}
}
