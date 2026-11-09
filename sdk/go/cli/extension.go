package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/numinance/numinance-spec/sdk/go/extpkg"
	kindsv1 "github.com/numinance/numinance-spec/sdk/go/gen/numinance/kinds/v1"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

const extensionUsage = `usage: numi extension verify [flags]

Checks, offline, that every extension the lockfile pins is installed exactly as pinned: the archive in the store hashes
as the entry's zh, its unpacked tree as h1, and its content as digest. It does not check signatures, the permission set
or revocation; those need the install flow and the signed index.

  --lockfile FILE   the lockfile (default .numinance.lock.yaml)
  --store DIR       the installed archives, <id>/<version>.zip below it (default .numinance/extensions)
  --json            print one JSON object
`

// maxArchiveBytes bounds one archive read by "numi extension verify".
const maxArchiveBytes = 256 << 20

// extensionResult is the verdict on one lock entry. Fields names the pins that differ, never their values.
type extensionResult struct {
	ID      string   `json:"id"`
	Version string   `json:"version"`
	OK      bool     `json:"ok"`
	Fields  []string `json:"fields,omitempty"`
	Reason  string   `json:"reason,omitempty"`
}

// RunExtension runs "numi extension ...": args are the arguments after "extension". Only "verify" exists in the open
// build; "add" and "update" go through the Engine (the install is a governed action).
func RunExtension(_ context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		_, _ = fmt.Fprint(stderr, extensionUsage)
		return ExitUsage
	}
	fl := flag.NewFlagSet("numi extension verify", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	lock := fl.String("lockfile", ".numinance.lock.yaml", "")
	store := fl.String("store", ".numinance/extensions", "")
	asJSON := fl.Bool("json", false, "")
	if err := fl.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(stdout, extensionUsage)
			return ExitOK
		}
		_, _ = fmt.Fprintf(stderr, "numi extension verify: %s\n%s", clean(err.Error(), maxMessageRunes), extensionUsage)
		return ExitUsage
	}
	if fl.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "numi extension verify: unexpected arguments\n%s", extensionUsage)
		return ExitUsage
	}
	entries, err := readLockfile(*lock)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "numi extension verify: %s\n", printable(err))
		return exitOf(err)
	}
	results := verifyEntries(*store, entries)
	failed := 0
	for _, r := range results {
		if !r.OK {
			failed++
		}
	}
	if *asJSON {
		if err := json.NewEncoder(stdout).Encode(map[string]any{"ok": failed == 0, "extensions": results}); err != nil {
			_, _ = fmt.Fprintf(stderr, "numi extension verify: the report cannot be encoded: %v\n", err)
			return ExitInternal
		}
	} else {
		for _, r := range results {
			printResult(stdout, r)
		}
		_, _ = fmt.Fprintf(stdout, "%d extensions, %d verified, %d failed\n", len(results), len(results)-failed, failed)
	}
	if failed > 0 {
		return ExitConflict
	}
	return ExitOK
}

func printResult(w io.Writer, r extensionResult) {
	if r.OK {
		_, _ = fmt.Fprintf(w, "ok      %s %s\n", quote(r.ID), quote(r.Version))
		return
	}
	detail := r.Reason
	if len(r.Fields) > 0 {
		detail = "differs: " + strings.Join(r.Fields, ", ")
	}
	_, _ = fmt.Fprintf(w, "FAILED  %s %s: %s\n", quote(r.ID), quote(r.Version), detail)
}

func exitOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return ExitCode(e.Kind)
	}
	return ExitInternal
}

func readLockfile(path string) ([]*kindsv1.LockedExtension, error) {
	b, err := readBounded(path, kindyaml.MaxBytes)
	if err != nil {
		return nil, safeErrorf(KindNotFound, "the lockfile %s cannot be read", quote(path))
	}
	l, err := kindyaml.NewLoader()
	if err != nil {
		return nil, Errorf(KindInternal, "kind loader: %v", err)
	}
	m, err := l.Decode(b)
	if err != nil {
		return nil, safeErrorf(KindInvalid, "the lockfile %s is not a valid Lockfile: %s", quote(path), quote(err.Error()))
	}
	doc, ok := m.(*kindsv1.Lockfile)
	if !ok {
		return nil, safeErrorf(KindInvalid, "%s is not a Lockfile", quote(path))
	}
	return doc.GetSpec().GetExtensions(), nil
}

// readBounded reads a regular file of at most limit bytes. A symlink is refused: the lockfile and the store are the
// repository's own files.
func readBounded(path string, limit int64) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fs.ErrInvalid
	}
	f, err := os.Open(path) //nolint:gosec // the path is the caller's own flag, checked above to be a regular file
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }() // a read-only handle: nothing to lose on close
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fs.ErrInvalid
	}
	return b, nil
}

func verifyEntries(store string, entries []*kindsv1.LockedExtension) []extensionResult {
	out := make([]extensionResult, 0, len(entries))
	for _, e := range entries {
		out = append(out, verifyEntry(store, e))
	}
	return out
}

func verifyEntry(store string, e *kindsv1.LockedExtension) extensionResult {
	r := extensionResult{ID: e.GetId(), Version: e.GetVersion()}
	// The loader checked the id and the version as names, so they are safe path segments.
	archive, err := readBounded(store+"/"+e.GetId()+"/"+e.GetVersion()+".zip", maxArchiveBytes)
	if err != nil {
		r.Reason = "the installed archive is missing or unreadable"
		return r
	}
	files, err := extpkg.Unpack(archive, extpkg.DefaultLimits())
	if err != nil {
		r.Reason = "the installed archive is not a valid package"
		return r
	}
	if extpkg.ArchiveHash(archive) != e.GetZh() {
		r.Fields = append(r.Fields, "zh")
	}
	if extpkg.TreeHash(files) != e.GetH1() {
		r.Fields = append(r.Fields, "h1")
	}
	if extpkg.ContentDigest(files) != e.GetDigest().GetValue() {
		r.Fields = append(r.Fields, "digest")
	}
	r.OK = len(r.Fields) == 0
	return r
}
