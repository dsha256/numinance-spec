package evidenceverify

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"unicode"
)

// MaxKeyFile is the most bytes a key file may hold.
const MaxKeyFile = 1024

// Options are the caller's choices. The caller must choose exactly one key
// mode: pinned runner keys, TrustBundleKeys or SkipSignatures.
type Options struct {
	// RunnerKeys are the pinned runner public keys. Any of them may sign a
	// record, the attestation and the manifest: evidence/v1 has no per-role key.
	RunnerKeys []ed25519.PublicKey
	// TrustBundleKeys trusts the keys that provenance/signers.json states. The
	// best level is self-consistent.
	TrustBundleKeys bool
	// SkipSignatures checks no signature. The best level is unauthenticated.
	SkipSignatures bool

	// LogKey and LogOrigin pin the transparency log; both or neither.
	LogKey    ed25519.PublicKey
	LogOrigin string

	// AllowUnanchored accepts a head not proved under a pinned log.
	AllowUnanchored bool
	// AllowIncomplete accepts a chain with no terminal record.
	AllowIncomplete bool
	// AllowUnsigned accepts the export of an Abandoned or RolledBack change.
	AllowUnsigned bool
	// Complete fails when a part of the design list is absent.
	Complete bool

	// ExpectChange, when set, must be the change id of the chain.
	ExpectChange string
	// ExpectHead, when set, must be the last record's hash or the anchored hash.
	ExpectHead string
}

// Validate checks the options and returns a KindUsage error for a bad choice.
func (o Options) Validate() error {
	modes := 0
	if len(o.RunnerKeys) > 0 {
		modes++
	}
	if o.TrustBundleKeys {
		modes++
	}
	if o.SkipSignatures {
		modes++
	}
	if modes != 1 {
		return errf(KindUsage, "choose exactly one key mode: --runner-key, --trust-bundle-keys or --skip-signatures")
	}
	for _, k := range o.RunnerKeys {
		if len(k) != ed25519.PublicKeySize {
			return errf(KindUsage, "a runner key is not 32 bytes")
		}
	}
	if (len(o.LogKey) > 0) != (o.LogOrigin != "") {
		return errf(KindUsage, "--log-key and --log-origin go together")
	}
	if len(o.LogKey) > 0 && len(o.LogKey) != ed25519.PublicKeySize {
		return errf(KindUsage, "the log key is not 32 bytes")
	}
	if o.ExpectChange != "" && !isChangeID(o.ExpectChange) {
		return errf(KindUsage, "--expect-change is not a change id")
	}
	if o.ExpectHead != "" && !isDigest(o.ExpectHead) {
		return errf(KindUsage, "--expect-head is not a digest")
	}
	return nil
}

// ParseKey reads a public key from the text of a key file: 64 hexadecimal
// characters of either case, with white space (any Unicode white space)
// trimmed from both ends.
func ParseKey(text []byte) (ed25519.PublicKey, error) {
	s := strings.TrimFunc(string(text), unicode.IsSpace)
	if len(s) != 2*ed25519.PublicKeySize {
		return nil, errors.New("a key is 64 hexadecimal characters")
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.New("a key is 64 hexadecimal characters")
	}
	return ed25519.PublicKey(raw), nil
}

// ReadKeyFile reads a key file of at most MaxKeyFile bytes. A longer file is an
// error and is never read as a truncated key. Every failure is a KindUsage
// error.
func ReadKeyFile(path string) (ed25519.PublicKey, error) {
	f, err := os.Open(path) //nolint:gosec // the key file is named by the caller
	if err != nil {
		return nil, errf(KindUsage, "a key file cannot be read")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, MaxKeyFile+1))
	if err != nil || len(data) > MaxKeyFile {
		return nil, errf(KindUsage, "a key file cannot be read or is over %d bytes", MaxKeyFile)
	}
	k, err := ParseKey(data)
	if err != nil {
		return nil, errf(KindUsage, "a key file does not hold a key: %v", err)
	}
	return k, nil
}
