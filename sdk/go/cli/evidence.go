package cli

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/numinance/numinance-spec/sdk/go/evidenceverify"
)

const evidenceUsage = `usage: numi evidence verify [flags] DIR

  --runner-key FILE        a pinned runner public key (64 hex characters); repeatable
  --trust-bundle-keys      trust the keys provenance/signers.json states (best level: self-consistent)
  --skip-signatures        check no signature (best level: unauthenticated)
  --log-key FILE           the pinned transparency log public key (with --log-origin)
  --log-origin ORIGIN      the pinned transparency log origin (with --log-key)
  --allow-unanchored       accept a head not proved under a pinned log
  --allow-incomplete       accept a chain with no terminal record
  --allow-unsigned         accept the export of an Abandoned or RolledBack change
  --complete               fail when a part of the design list is absent
  --expect-change CHG_ID   fail unless the chain is this change
  --expect-head DIGEST     fail unless this is the last record's hash or the anchored hash
  --json                   print one JSON object

Exactly one key mode is required. A key the export states never reaches "verified".
`

// RunEvidence runs "numi evidence ...": args are the arguments after "evidence".
// It is a thin wrapper over the open verifier in package evidenceverify, which
// reads the directory offline and read-only. It returns the exit code of the
// exit-code table of docs/evidence/verification.md section 5.
func RunEvidence(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		_, _ = fmt.Fprint(stderr, evidenceUsage)
		return ExitUsage
	}
	opts, dirs, asJSON, err := parseVerifyArgs(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprint(stdout, evidenceUsage)
		return ExitOK
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "numi evidence verify: %v\n%s", err, evidenceUsage)
		return ExitUsage
	}
	if len(dirs) != 1 {
		_, _ = fmt.Fprintf(stderr, "numi evidence verify: exactly one directory is required\n%s", evidenceUsage)
		return ExitUsage
	}
	rep, err := evidenceverify.Verify(ctx, dirs[0], opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "numi evidence verify: %v\n", err)
		return evidenceverify.ExitCode(nil, err)
	}
	if asJSON {
		if err := json.NewEncoder(stdout).Encode(rep); err != nil {
			_, _ = fmt.Fprintf(stderr, "numi evidence verify: the report cannot be encoded: %v\n", err)
			return ExitInternal
		}
	} else {
		_, _ = fmt.Fprint(stdout, rep.Text())
	}
	return evidenceverify.ExitCode(rep, nil)
}

// parseVerifyArgs reads the flags of verify. Flags and the directory may come
// in any order; "--" ends the flags.
func parseVerifyArgs(args []string) (evidenceverify.Options, []string, bool, error) {
	var (
		opts       evidenceverify.Options
		runnerKeys []string
		logKey     string
		asJSON     bool
		positional []string
		keyErr     error
		fs         = flag.NewFlagSet("verify", flag.ContinueOnError)
	)
	fs.SetOutput(io.Discard)
	fs.Func("runner-key", "", func(s string) error { runnerKeys = append(runnerKeys, s); return nil })
	fs.BoolVar(&opts.TrustBundleKeys, "trust-bundle-keys", false, "")
	fs.BoolVar(&opts.SkipSignatures, "skip-signatures", false, "")
	fs.StringVar(&logKey, "log-key", "", "")
	fs.StringVar(&opts.LogOrigin, "log-origin", "", "")
	fs.BoolVar(&opts.AllowUnanchored, "allow-unanchored", false, "")
	fs.BoolVar(&opts.AllowIncomplete, "allow-incomplete", false, "")
	fs.BoolVar(&opts.AllowUnsigned, "allow-unsigned", false, "")
	fs.BoolVar(&opts.Complete, "complete", false, "")
	fs.StringVar(&opts.ExpectChange, "expect-change", "", "")
	fs.StringVar(&opts.ExpectHead, "expect-head", "", "")
	fs.BoolVar(&asJSON, "json", false, "")

	if i := slices.Index(args, "--"); i >= 0 {
		positional = append(positional, args[i+1:]...)
		args = args[:i]
	}
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return opts, nil, false, err
		}
		args = fs.Args()
		if len(args) > 0 {
			positional = append(positional, args[0])
			args = args[1:]
		}
	}
	for _, p := range runnerKeys {
		var k ed25519.PublicKey
		if k, keyErr = evidenceverify.ReadKeyFile(p); keyErr != nil {
			return opts, nil, false, keyErr
		}
		opts.RunnerKeys = append(opts.RunnerKeys, k)
	}
	if logKey != "" {
		k, err := evidenceverify.ReadKeyFile(logKey)
		if err != nil {
			return opts, nil, false, err
		}
		opts.LogKey = k
	}
	if err := opts.Validate(); err != nil {
		return opts, nil, false, err
	}
	return opts, positional, asJSON, nil
}
