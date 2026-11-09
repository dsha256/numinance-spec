package evidenceverify

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const root = "../../../conformance/evidence/v1"

func mustKey(t *testing.T, name string) ed25519.PublicKey {
	t.Helper()
	k, err := ReadKeyFile(filepath.Join(root, "keys", name))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func pinned(t *testing.T) Options {
	t.Helper()
	return Options{RunnerKeys: []ed25519.PublicKey{mustKey(t, "runner.pub")}, LogKey: mustKey(t, "log.pub"), LogOrigin: "numinance.test/log"}
}

func exportDir(name string) string { return filepath.Join(root, "exports", name) }

func TestVerify_TooManyRecords_AtTheEdge(t *testing.T) {
	// closed-basic holds 4 records: a bound of 4 passes, a bound of 3 fails
	// before the fourth record is kept.
	tests := []struct {
		name string
		max  uint64
		want []string
	}{
		{name: "bound equals the chain length", max: 4, want: []string{}},
		{name: "bound one below the chain length", max: 3, want: []string{"too_many_records"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := verify(context.Background(), exportDir("closed-basic"), pinned(t), tc.max)
			if err != nil {
				t.Fatal(err)
			}
			if got := rep.FindingCodes(); !slices.Equal(got, nonNilStrings(tc.want)) {
				t.Errorf("finding codes = %v, want %v", got, tc.want)
			}
		})
	}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func TestVerify_Context(t *testing.T) {
	tests := []struct {
		name     string
		newCtx   func() (context.Context, context.CancelFunc)
		wantExit int
	}{
		{name: "an expired deadline is a timeout", wantExit: 8, newCtx: func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}},
		{name: "a cancel is an internal error", wantExit: 1, newCtx: func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.newCtx()
			defer cancel()
			rep, err := Verify(ctx, exportDir("closed-basic"), pinned(t))
			if got := ExitCode(rep, err); got != tc.wantExit {
				t.Errorf("exit code = %d, want %d (err %v)", got, tc.wantExit, err)
			}
		})
	}
}

func TestOptions_Validate(t *testing.T) {
	key := mustKey(t, "runner.pub")
	log := mustKey(t, "log.pub")
	const goodChange = "chg_01010101010101010101010101010101"
	tests := []struct {
		name string
		opts Options
		ok   bool
	}{
		{name: "one pinned key", opts: Options{RunnerKeys: []ed25519.PublicKey{key}}, ok: true},
		{name: "two pinned keys are one mode", opts: Options{RunnerKeys: []ed25519.PublicKey{key, log}}, ok: true},
		{name: "no key mode", opts: Options{}},
		{name: "pinned and skip", opts: Options{RunnerKeys: []ed25519.PublicKey{key}, SkipSignatures: true}},
		{name: "trust and skip", opts: Options{TrustBundleKeys: true, SkipSignatures: true}},
		{name: "log key without origin", opts: Options{SkipSignatures: true, LogKey: log}},
		{name: "log origin without key", opts: Options{SkipSignatures: true, LogOrigin: "x"}},
		{name: "short runner key", opts: Options{RunnerKeys: []ed25519.PublicKey{key[:5]}}},
		{name: "expect-change well formed", opts: Options{SkipSignatures: true, ExpectChange: goodChange}, ok: true},
		{name: "expect-change malformed", opts: Options{SkipSignatures: true, ExpectChange: "chg_1"}},
		{name: "expect-head malformed", opts: Options{SkipSignatures: true, ExpectHead: "sha256:ABC"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate()
			if (err == nil) != tc.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && KindOf(err) != KindUsage {
				t.Errorf("kind = %v, want usage", KindOf(err))
			}
		})
	}
}

func TestParseKey(t *testing.T) {
	hex := strings.Repeat("ab", 32)
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{name: "lower case", in: hex, ok: true},
		{name: "upper case", in: strings.ToUpper(hex), ok: true},
		{name: "ASCII white space around", in: " \n" + hex + "\t\n", ok: true},
		{name: "no-break space around", in: " " + hex + " ", ok: true},
		{name: "63 characters", in: hex[:63]},
		{name: "65 characters", in: hex + "a"},
		{name: "not hex", in: strings.Repeat("zz", 32)},
		{name: "inner space", in: hex[:10] + " " + hex[10:]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseKey([]byte(tc.in))
			if (err == nil) != tc.ok {
				t.Errorf("ParseKey(%q) error = %v, want ok=%v", tc.in, err, tc.ok)
			}
		})
	}
}

func TestReadKeyFile_OversizeIsNeverTruncated(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "k")
	// A valid key followed by padding past 1024 bytes: refused, not read short.
	if err := os.WriteFile(p, []byte(strings.Repeat("ab", 32)+strings.Repeat(" ", MaxKeyFile)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadKeyFile(p); KindOf(err) != KindUsage {
		t.Errorf("ReadKeyFile(oversize) = %v, want a usage error", err)
	}
}

func TestCanonical_Strings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "html is not escaped", in: "<a>&", want: `"<a>&"`},
		{name: "quote and backslash", in: "\"\\", want: `"\"\\"`},
		{name: "short escapes", in: "\n\r\t\b\f", want: `"\n\r\t\b\f"`},
		{name: "other control character", in: "\x01\x1f", want: `"\u0001\u001f"`},
		{name: "del is raw", in: "\x7f", want: "\"\x7f\""},
		{name: "line separator", in: "\u2028\u2029", want: "\"\\u2028\\u2029\""},
		{name: "non-ASCII is raw", in: "é€", want: `"é€"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(canonical(tc.in)); got != tc.want {
				t.Errorf("canonical(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseJSON_Refusals(t *testing.T) {
	tests := []struct {
		name string
		in   string
		ok   bool
	}{
		{name: "object", in: `{"a":1}`, ok: true},
		{name: "duplicate key", in: `{"a":1,"a":2}`},
		{name: "trailing data", in: `{"a":1} `},
		{name: "white space", in: `{ "a":1}`},
		{name: "invalid UTF-8", in: "{\"a\":\"\xff\"}"},
		{name: "lone surrogate", in: `{"a":"\ud800"}`},
		{name: "surrogate pair", in: `{"a":"😀"}`, ok: true},
		{name: "raw control character", in: "{\"a\":\"\x01\"}"},
		{name: "deeper than 64", in: strings.Repeat("[", 70) + strings.Repeat("]", 70)},
		{name: "empty", in: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseJSON([]byte(tc.in))
			if (err == nil) != tc.ok {
				t.Errorf("parseJSON(%q) error = %v, want ok=%v", tc.in, err, tc.ok)
			}
		})
	}
}

func TestStreamDigests(t *testing.T) {
	s1 := "{\"seq\":1}\n{\"seq\":2}\n"
	s2 := "{\"seq\":1,\"x\":1}\n"
	tests := []struct {
		name string
		in   string
		want []string
		ok   bool
	}{
		{name: "empty file is no stream", in: "", want: nil, ok: true},
		{name: "one stream", in: s1, want: []string{sha256Digest([]byte(s1))}, ok: true},
		{name: "two streams split at seq 1", in: s1 + s2, want: []string{sha256Digest([]byte(s1)), sha256Digest([]byte(s2))}, ok: true},
		{name: "key matches without regard to case", in: "{\"SEQ\":1}\n", want: []string{sha256Digest([]byte("{\"SEQ\":1}\n"))}, ok: true},
		{name: "last duplicate wins", in: "{\"seq\":2,\"seq\":1}\n", want: []string{sha256Digest([]byte("{\"seq\":2,\"seq\":1}\n"))}, ok: true},
		{name: "first line not seq 1", in: "{\"seq\":2}\n"},
		{name: "no final line feed", in: "{\"seq\":1}"},
		{name: "blank line", in: "{\"seq\":1}\n\n"},
		{name: "fraction", in: "{\"seq\":1.0}\n"},
		{name: "negative", in: "{\"seq\":-1}\n"},
		{name: "not an object", in: "[1]\n"},
		{name: "missing seq", in: "{}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := streamDigests([]byte(tc.in))
			if ok != tc.ok || !slices.Equal(got, tc.want) {
				t.Errorf("streamDigests() = %v, %v, want %v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestProof_ShapeAndRoot(t *testing.T) {
	// A tree of one leaf has an empty path; its root is the leaf hash.
	leaf := headLeaf(leafHead, make([]byte, 32))
	p := &proof{leafIndex: 0, treeSize: 1}
	if r, ok := p.root(leaf); !ok || string(r) != string(leafHash(leaf)) {
		t.Errorf("root of a one-leaf tree = %x, %v", r, ok)
	}
	// Two leaves: the root is node_hash(l0, l1) and each side proves the other.
	l0, l1 := headLeaf(leafHead, make([]byte, 32)), headLeaf(leafAttestation, make([]byte, 32))
	want := nodeHash(leafHash(l0), leafHash(l1))
	for i, tc := range []struct {
		leaf  []byte
		index uint64
		other []byte
	}{{l0, 0, leafHash(l1)}, {l1, 1, leafHash(l0)}} {
		q := &proof{leafIndex: tc.index, treeSize: 2, path: [][]byte{tc.other}}
		if r, ok := q.root(tc.leaf); !ok || string(r) != string(want) {
			t.Errorf("case %d: root = %x, %v, want %x", i, r, ok, want)
		}
	}
	// A path that is too long for the tree is refused.
	long := &proof{leafIndex: 0, treeSize: 1, path: [][]byte{make([]byte, 32)}}
	if _, ok := long.root(leaf); ok {
		t.Error("a path longer than the tree implies verified")
	}
}

func TestReport_JSONKeys(t *testing.T) {
	rep, err := Verify(context.Background(), exportDir("closed-sealed"), pinned(t))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ok", "change_id", "records", "head_record_hash", "anchored_head_record_hash", "ended", "terminal_state", "sealed", "level", "level_enum", "weakened_by_flags", "chain", "signatures", "anchor", "attestation", "credential_scopes", "network_policy"} {
		if _, ok := m[k]; !ok {
			t.Errorf("report JSON has no key %q", k)
		}
	}
	if m["head_record_hash"] == m["anchored_head_record_hash"] {
		t.Error("a sealed chain: the last record's hash must differ from the anchored hash")
	}
	if got, ok := m["weakened_by_flags"].([]any); !ok || len(got) != 0 {
		t.Errorf("weakened_by_flags = %v, want an empty array", m["weakened_by_flags"])
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		rep  *Report
		err  error
		want int
	}{
		{name: "pass", rep: &Report{LevelEnum: LevelVerified}, want: 0},
		{name: "pass with an allowance", rep: &Report{LevelEnum: LevelVerified, WeakenedByFlags: []string{SuffixUnanchored}}, want: 0},
		{name: "finding", rep: &Report{LevelEnum: LevelFailed, Findings: []Finding{{Code: CodeChainBroken}}}, want: 3},
		{name: "weak", rep: &Report{LevelEnum: LevelUnanchored}, want: 5},
		{name: "usage", err: errf(KindUsage, "x"), want: 2},
		{name: "refused path", err: errf(KindInvalid, "x"), want: 3},
		{name: "not found", err: errf(KindNotFound, "x"), want: 4},
		{name: "unavailable", err: errf(KindUnavailable, "x"), want: 7},
		{name: "timeout", err: errf(KindTimeout, "x"), want: 8},
		{name: "deadline", err: context.DeadlineExceeded, want: 8},
		{name: "other", err: os.ErrClosed, want: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCode(tc.rep, tc.err); got != tc.want {
				t.Errorf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestPackage_StdlibOnlyAndOffline is the independence and trust-boundary
// guard: the package imports nothing but the standard library (so never
// numinance/ or a third-party module), and no networking or process package.
func TestPackage_StdlibOnlyAndOffline(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"net", "net/http", "os/exec", "plugin", "unsafe", "syscall"}
	for _, f := range files {
		af, err := goparser.ParseFile(token.NewFileSet(), f, nil, goparser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range af.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			first, _, _ := strings.Cut(path, "/")
			selfImport := strings.HasSuffix(f, "_test.go") && path == "github.com/numinance/numinance-spec/sdk/go/evidenceverify"
			if strings.Contains(first, ".") && !selfImport {
				t.Errorf("%s imports %s: the package is standard library only", f, path)
			}
			if slices.Contains(banned, path) || strings.HasPrefix(path, "net/") {
				t.Errorf("%s imports %s: the verifier is offline and read-only", f, path)
			}
		}
	}
}

// TestVerify_MutatedFilesNeverPanicOrPass corrupts each file of two exports in
// three ways. The verifier must return a report (never panic) and the level
// must not be a pass: every file is covered by the index, the manifest or the
// chain.
func TestVerify_MutatedFilesNeverPanicOrPass(t *testing.T) {
	mutations := []struct {
		name string
		fn   func([]byte) []byte
	}{
		{name: "truncated to half", fn: func(b []byte) []byte { return b[:len(b)/2] }},
		{name: "first byte flipped", fn: func(b []byte) []byte { c := slices.Clone(b); c[0] ^= 0x01; return c }},
		{name: "garbage appended", fn: func(b []byte) []byte { return append(slices.Clone(b), "\x00\xff{"...) }},
	}
	for _, export := range []string{"closed-complete", "closed-sealed"} {
		src := exportDir(export)
		var files []string
		err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(src, p)
				files = append(files, rel)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, rel := range files {
			orig, err := os.ReadFile(filepath.Join(src, rel)) //nolint:gosec // a vector file listed by the walk
			if err != nil || len(orig) == 0 {
				continue
			}
			for _, m := range mutations {
				t.Run(export+"/"+rel+"/"+m.name, func(t *testing.T) {
					dir := t.TempDir()
					copyExport(t, src, dir)
					if err := os.WriteFile(filepath.Join(dir, rel), m.fn(orig), 0o600); err != nil { //nolint:gosec // under the test temp directory
						t.Fatal(err)
					}
					rep, err := Verify(context.Background(), dir, pinned(t))
					if err != nil {
						t.Fatal(err)
					}
					if rep.ExitCode() == 0 {
						t.Errorf("a mutated %s passed as %q", rel, rep.Level)
					}
				})
			}
		}
	}
}

func copyExport(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o750)
		}
		b, err := os.ReadFile(p) //nolint:gosec // a vector file listed by the walk
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o600) //nolint:gosec // under the test temp directory
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerify_SymlinkedPathIsRefusedWhateverTheSpelling(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "export")
	copyExport(t, exportDir("closed-basic"), realDir)
	link := filepath.Join(base, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("no symbolic links: %v", err)
	}
	sep := string(filepath.Separator)
	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "the link", path: link, want: 3},
		{name: "trailing separator", path: link + sep, want: 3},
		{name: "double trailing separator", path: link + sep + sep, want: 3},
		{name: "trailing dot", path: link + sep + ".", want: 3},
		{name: "the realDir directory", path: realDir, want: 0},
		{name: "the realDir directory with a trailing separator", path: realDir + sep, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rep, err := Verify(context.Background(), tc.path, pinned(t))
			if got := ExitCode(rep, err); got != tc.want {
				t.Errorf("Verify(%q) exit code = %d, want %d (err %v)", tc.path, got, tc.want, err)
			}
			if tc.want == 3 && rep != nil {
				t.Errorf("a refused path produced a report: %s", rep.Level)
			}
		})
	}
}

func TestVerify_EmptyPathIsAUsageError(t *testing.T) {
	// filepath.Clean("") is ".": without a refusal an empty argument would verify the working directory, and here the
	// working directory is a valid export.
	opts := pinned(t)
	t.Chdir(exportDir("closed-basic"))
	rep, err := Verify(context.Background(), "", opts)
	if rep != nil || KindOf(err) != KindUsage {
		t.Errorf("Verify(%q) = %v, %v, want no report and a usage error", "", rep, err)
	}
	if got := ExitCode(rep, err); got != 2 {
		t.Errorf("ExitCode(Verify(%q)) = %d, want 2", "", got)
	}
}

func TestFallbacks_ValidRecordNeedsDecisionGrantAndAFreshRun(t *testing.T) {
	failed := &record{sequence: 1, action: actionStream, hash: "h1", runID: "run-1", inputs: []artifact{{ref: refConsidered}}}
	route := func(edit func(r *record)) *record {
		r := &record{
			sequence: 2, action: actionRoute, hash: "h2", runID: "run-2", kind: "runner", outcome: "succeeded", decisionID: "dec-1", grantID: "grant-1",
			inputs: []artifact{{ref: refConsidered}, {ref: refFallbackOf, sha256: "h1"}, {ref: refFallbackCause, sha256: sha256Digest([]byte(causeExecutorFail))}},
		}
		edit(r)
		return r
	}
	tests := []struct {
		name string
		span []*record
		want int
	}{
		{"a complete fallback record", []*record{failed, route(func(*record) {})}, 1},
		{"no policy decision", []*record{failed, route(func(r *record) { r.decisionID = "" })}, 0},
		{"no grant", []*record{failed, route(func(r *record) { r.grantID = "" })}, 0},
		{"a run a record of lower sequence already has", []*record{{sequence: 0, action: "scm.read", runID: "run-2"}, failed, route(func(*record) {})}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(fallbacks(tt.span)); got != tt.want {
				t.Errorf("len(fallbacks()) = %d, want %d", got, tt.want)
			}
		})
	}
}
