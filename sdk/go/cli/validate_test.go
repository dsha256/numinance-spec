package cli_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

//nolint:gochecknoglobals // standard golden-file flag; flags cannot be declared locally in tests
var update = flag.Bool("update", false, "rewrite golden files")

const goToolchain = `apiVersion: numinance.dev/v1
kind: LanguageToolchain
metadata: {name: go-1.27, scope: SCOPE_ORG}
spec:
  language: go
  version: "1.27"
  verifiers:
    - {name: unit-tests, provider: dev.numinance.official/go-test}
  formatters: [gofmt]
  package_manager: go-modules
`

type fakeSource struct {
	snap cli.Snapshot
	err  error
	got  cli.Request
}

func (f *fakeSource) Snapshot(_ context.Context, req cli.Request) (cli.Snapshot, error) {
	f.got = req
	return f.snap, f.err
}

type fakeResolver struct {
	res  cli.Result
	err  error
	got  cli.Request
	snap cli.Snapshot
}

func (f *fakeResolver) Resolve(_ context.Context, req cli.Request, snap cli.Snapshot) (cli.Result, error) {
	f.got, f.snap = req, snap
	return f.res, f.err
}

func oneDoc(data string) *fakeSource {
	return &fakeSource{snap: cli.Snapshot{
		Revision:  "0123abcd",
		Documents: []cli.Document{{Scope: "org", Path: ".numinance/policy/go.yaml", Data: []byte(data)}},
	}}
}

func run(v cli.Validate, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	v.Stdout, v.Stderr = &out, &errb
	code = v.Run(context.Background(), args)
	return code, out.String(), errb.String()
}

func TestValidate_Run_SchemaErrorExitsInvalid(t *testing.T) {
	bad := strings.Replace(goToolchain, "language: go", "language: 7", 1)
	code, out, _ := run(cli.Validate{Source: oneDoc(bad), Resolver: &fakeResolver{}})
	if code != cli.ExitInvalid {
		t.Errorf("Run(schema error) = %d, want %d", code, cli.ExitInvalid)
	}
	if !strings.Contains(out, "invalid") {
		t.Errorf("stdout = %q, want an invalid line", out)
	}
}

func TestValidate_Run_UnknownFieldExitsInvalid(t *testing.T) {
	bad := goToolchain + "  surprise: 1\n"
	r := &fakeResolver{}
	code, _, _ := run(cli.Validate{Source: oneDoc(bad), Resolver: r})
	if code != cli.ExitInvalid {
		t.Errorf("Run(unknown field) = %d, want %d", code, cli.ExitInvalid)
	}
	if r.got != (cli.Request{}) {
		t.Errorf("resolver ran on an invalid document: %+v", r.got)
	}
}

func TestValidate_Run_UsageErrorExits2(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--bogus"}},
		{name: "two paths", args: []string{"a", "b"}},
		{name: "flag without value", args: []string{"--rev"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := oneDoc(goToolchain)
			code, _, _ := run(cli.Validate{Source: src}, tc.args...)
			if code != cli.ExitUsage {
				t.Errorf("Run(%v) = %d, want %d", tc.args, code, cli.ExitUsage)
			}
		})
	}
}

func TestValidate_Run_NoResolverExitsUnavailable(t *testing.T) {
	code, _, stderr := run(cli.Validate{Source: oneDoc(goToolchain)})
	if code != cli.ExitUnavailable {
		t.Errorf("Run(no resolver) = %d, want %d", code, cli.ExitUnavailable)
	}
	if !strings.Contains(stderr, "needs the Numinance engine build") {
		t.Errorf("stderr = %q, want the engine-build message", stderr)
	}
}

func TestValidate_Run_PassesFlagsAnywhere(t *testing.T) {
	r := &fakeResolver{res: cli.Result{Digest: "sha256:x"}}
	code, _, _ := run(cli.Validate{Source: oneDoc(goToolchain), Resolver: r}, "repo", "--rev", "main", "--component=svc/api", "--org", "../org")
	if code != cli.ExitOK {
		t.Fatalf("Run = %d, want 0", code)
	}
	want := cli.Request{RepoDir: "repo", OrgDir: "../org", Rev: "main", Component: "svc/api"}
	if r.got != want {
		t.Errorf("resolver request = %+v, want %+v", r.got, want)
	}
}

func TestValidate_Run_DefaultsToCurrentDirectoryAndSourceRevision(t *testing.T) {
	r := &fakeResolver{}
	run(cli.Validate{Source: oneDoc(goToolchain), Resolver: r})
	want := cli.Request{RepoDir: "."}
	if r.got != want {
		t.Errorf("resolver request = %+v, want %+v", r.got, want)
	}
}

func TestValidate_Run_MapsErrorKinds(t *testing.T) {
	tests := []struct {
		name string
		src  *fakeSource
		res  *fakeResolver
		want int
	}{
		{name: "source not found", src: &fakeSource{err: cli.Errorf(cli.KindNotFound, "no repo")}, res: &fakeResolver{}, want: cli.ExitNotFound},
		{name: "source unknown error", src: &fakeSource{err: errors.New("boom")}, res: &fakeResolver{}, want: cli.ExitInternal},
		{name: "source error without a kind", src: &fakeSource{err: &cli.Error{Msg: "x"}}, res: &fakeResolver{}, want: cli.ExitInternal},
		{name: "resolver invalid", src: oneDoc(goToolchain), res: &fakeResolver{err: cli.Errorf(cli.KindInvalid, "floor")}, want: cli.ExitInvalid},
		{name: "resolver timeout", src: oneDoc(goToolchain), res: &fakeResolver{err: context.DeadlineExceeded}, want: cli.ExitTimeout},
		{name: "no documents", src: &fakeSource{}, res: &fakeResolver{}, want: cli.ExitNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := run(cli.Validate{Source: tc.src, Resolver: tc.res})
			if code != tc.want {
				t.Errorf("Run(%s) = %d, want %d", tc.name, code, tc.want)
			}
		})
	}
}

func TestValidate_Run_UsesInjectedErrorKind(t *testing.T) {
	v := cli.Validate{
		Source:    &fakeSource{err: errors.New("denied")},
		ErrorKind: func(error) cli.Kind { return cli.KindPolicyDenied },
	}
	if code, _, _ := run(v); code != cli.ExitPolicyDenied {
		t.Errorf("Run = %d, want %d", code, cli.ExitPolicyDenied)
	}
}

func TestValidate_Run_JSONOutputShape(t *testing.T) {
	r := &fakeResolver{res: cli.Result{
		Digest: "sha256:abc",
		Provenance: []cli.FieldSource{
			{Field: "autonomy.default_level", Scope: "repo", Path: ".numinance/policy/autonomy.yaml"},
		},
	}}
	code, out, _ := run(cli.Validate{Source: oneDoc(goToolchain), Resolver: r}, "--json")
	if code != cli.ExitOK {
		t.Fatalf("Run = %d, want 0", code)
	}
	golden := filepath.Join("testdata", "validate_ok.json.golden")
	if *update {
		if err := os.WriteFile(golden, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden) //nolint:gosec // constant testdata path
	if err != nil {
		t.Fatal(err)
	}
	if out != string(want) {
		t.Errorf("JSON output =\n%s\nwant\n%s", out, want)
	}
}

func TestValidate_Run_QuotesUntrustedNames(t *testing.T) {
	name := "evil\x1b[31m\u202e" + strings.Repeat("x", 500)
	doc := strings.Replace(goToolchain, "name: go-1.27", "name: \"go\"", 1)
	src := oneDoc(doc)
	src.snap.Documents[0].Path = ".numinance/policy/a\nb\u202e" + strings.Repeat("p", 500) + ".yaml"
	// A name with control characters fails the loader; its message must not reach the output raw.
	bad := strings.Replace(goToolchain, "name: go-1.27", "name: \""+strings.NewReplacer("\x1b", `\e`, "\u202e", `\u202e`).Replace(name)+"\"", 1)
	src.snap.Documents = append(src.snap.Documents, cli.Document{Scope: "repo", Path: src.snap.Documents[0].Path, Data: []byte(bad)})
	_, out, errOut := run(cli.Validate{Source: src, Resolver: &fakeResolver{}})
	for _, s := range []string{out, errOut} {
		for _, r := range s {
			if r == '\x1b' || r == '\u202e' {
				t.Fatalf("output has a raw control or bidi character: %q", s)
			}
		}
		if strings.Contains(s, strings.Repeat("p", 200)) {
			t.Errorf("output has an over-long path: %q", s)
		}
	}
	if !strings.Contains(out, `\u000a`) || !strings.Contains(out, `\u202e`) || !strings.Contains(out, "(cut)") {
		t.Errorf("stdout = %q, want escaped characters and a cut marker", out)
	}
}

func TestValidate_Run_ClassifierReturningNoneStillFails(t *testing.T) {
	v := cli.Validate{
		Source:    &fakeSource{err: errors.New("x")},
		ErrorKind: func(error) cli.Kind { return cli.KindNone },
	}
	if code, _, _ := run(v); code != cli.ExitInternal {
		t.Errorf("Run(classifier returns KindNone) = %d, want %d", code, cli.ExitInternal)
	}
}

type panicSource struct{}

func (panicSource) Snapshot(context.Context, cli.Request) (cli.Snapshot, error) { panic("boom") }

func TestValidate_Run_PanicExitsInternal(t *testing.T) {
	if code, _, _ := run(cli.Validate{Source: panicSource{}}); code != cli.ExitInternal {
		t.Errorf("Run(panicking source) = %d, want %d", code, cli.ExitInternal)
	}
}

func TestValidate_Run_SchemaErrorsDoNotPrintValues(t *testing.T) {
	bad := strings.Replace(goToolchain, "language: go", "language: CANARY-s3cr3t", 1) + "  CANARYkey: CANARY-value\n"
	code, out, errOut := run(cli.Validate{Source: oneDoc(bad)}, "--json")
	if code != cli.ExitInvalid {
		t.Fatalf("Run = %d, want %d", code, cli.ExitInvalid)
	}
	if strings.Contains(out, "s3cr3t") || strings.Contains(out, "CANARY-value") || strings.Contains(errOut, "s3cr3t") {
		t.Errorf("output has a document value: %q %q", out, errOut)
	}
}

func TestValidate_Run_HumanLinesAreExactAndOneToOne(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "newline quote backslash bidi and BEL", path: "a\n\"b\\c\u202e\a", want: `ok       org "a\u000a\"b\\c\u202e\u0007" LanguageToolchain "go-1.27"`},
		{name: "BEL", path: "a\a", want: `ok       org "a\u0007" LanguageToolchain "go-1.27"`},
		{name: "the six characters backslash u 0007", path: `a\u0007`, want: `ok       org "a\\u0007" LanguageToolchain "go-1.27"`},
	}
	seen := map[string]string{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := oneDoc(goToolchain)
			src.snap.Documents[0].Path = tc.path
			_, out, _ := run(cli.Validate{Source: src, Resolver: &fakeResolver{}})
			want := "revision: \"0123abcd\"\n" + tc.want + "\n"
			if out != want {
				t.Errorf("stdout for path %q =\n%s\nwant\n%s", tc.path, out, want)
			}
			_, js, _ := run(cli.Validate{Source: src, Resolver: &fakeResolver{}}, "--json")
			if prev, dup := seen[js]; dup {
				t.Errorf("--json output of path %q equals that of %q", tc.path, prev)
			}
			seen[js] = tc.path
		})
	}
}

func TestValidate_Run_NilWritersDoNotPanic(t *testing.T) {
	for _, args := range [][]string{nil, {"--json"}, {"--bogus"}, {"--help"}} {
		v := cli.Validate{Source: oneDoc(goToolchain), Resolver: &fakeResolver{}}
		v.Run(context.Background(), args) // a panic fails the test
	}
	cli.Root{}.Run(context.Background(), []string{"validate", "--bogus"})
	cli.Root{}.Run(context.Background(), []string{"nope"})
}

func TestValidate_Run_HelpExitsZero(t *testing.T) {
	code, out, _ := run(cli.Validate{Source: oneDoc(goToolchain)}, "--help")
	if code != cli.ExitOK || !strings.Contains(out, "usage: numi validate") {
		t.Errorf("Run(--help) = %d, stdout %q, want 0 and the usage text", code, out)
	}
}

func TestValidate_Run_UsageErrorEscapesArgv(t *testing.T) {
	code, _, errOut := run(cli.Validate{Source: oneDoc(goToolchain)}, "--bad\x1b[31mflag")
	if code != cli.ExitUsage || strings.Contains(errOut, "\x1b") {
		t.Errorf("Run(--bad ESC flag) = %d, stderr %q, want 2 and no raw escape", code, errOut)
	}
}

func TestValidate_Run_CanceledContextIsNotUnavailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errb bytes.Buffer
	v := cli.Validate{Source: oneDoc(goToolchain), Stdout: &out, Stderr: &errb}
	if code := v.Run(ctx, nil); code != cli.ExitInternal {
		t.Errorf("Run(canceled context) = %d, want %d (never %d)", code, cli.ExitInternal, cli.ExitUnavailable)
	}
	v.ErrorKind = func(error) cli.Kind { return cli.KindUnavailable }
	if code := v.Run(ctx, nil); code == cli.ExitUnavailable {
		t.Errorf("Run(canceled context, classifier says Unavailable) = %d, want not %d", code, cli.ExitUnavailable)
	}
}

func TestValidate_Run_ResolverGetsTheValidatedSnapshot(t *testing.T) {
	src := oneDoc(goToolchain)
	r := &fakeResolver{}
	run(cli.Validate{Source: src, Resolver: r})
	if r.snap.Revision != src.snap.Revision || len(r.snap.Documents) != 1 || string(r.snap.Documents[0].Data) != goToolchain {
		t.Errorf("resolver snapshot = %+v, want the snapshot that passed the schema check", r.snap)
	}
}

func TestValidate_Run_DoubleDashEndsFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantReq  cli.Request
	}{
		{name: "path that looks like a flag", args: []string{"--", "-x"}, wantCode: cli.ExitOK, wantReq: cli.Request{RepoDir: "-x"}},
		{name: "flag after the terminator is a second path", args: []string{"--", "repo", "--json"}, wantCode: cli.ExitUsage},
		{name: "flags before the terminator still apply", args: []string{"--org", "o", "--", "-r"}, wantCode: cli.ExitOK, wantReq: cli.Request{RepoDir: "-r", OrgDir: "o"}},
		{name: "a value may be the terminator text", args: []string{"--org", "--", "repo"}, wantCode: cli.ExitOK, wantReq: cli.Request{RepoDir: "repo", OrgDir: "--"}},
		{name: "equals form", args: []string{"--org=o", "repo", "--component=c"}, wantCode: cli.ExitOK, wantReq: cli.Request{RepoDir: "repo", OrgDir: "o", Component: "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeResolver{}
			code, _, _ := run(cli.Validate{Source: oneDoc(goToolchain), Resolver: r}, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("Run(%v) = %d, want %d", tc.args, code, tc.wantCode)
			}
			if tc.wantCode == cli.ExitOK && r.got != tc.wantReq {
				t.Errorf("Run(%v) request = %+v, want %+v", tc.args, r.got, tc.wantReq)
			}
		})
	}
}

func TestValidate_Run_PrintsIgnoredProvenance(t *testing.T) {
	r := &fakeResolver{res: cli.Result{
		Digest: "sha256:abc",
		Provenance: []cli.FieldSource{
			{Field: "budget.tokens", Scope: "org", Path: ".numinance/policy/b.yaml"},
			{Field: "autonomy.default_level", Scope: "repo", Path: ".numinance/a.yaml", Ignored: "no authority\x1b[31m"},
		},
	}}
	_, out, _ := run(cli.Validate{Source: oneDoc(goToolchain), Resolver: r})
	want := "  \"autonomy.default_level\" <- repo \".numinance/a.yaml\" (ignored: \"no authority\\u001b[31m\")\n"
	if !strings.Contains(out, want) {
		t.Errorf("output =\n%q\nwant it to contain %q", out, want)
	}
	if strings.Contains(out, `"budget.tokens" <- org ".numinance/policy/b.yaml" (`) {
		t.Errorf("an effective field is marked ignored:\n%s", out)
	}
	_, js, _ := run(cli.Validate{Source: oneDoc(goToolchain), Resolver: r}, "--json")
	if !strings.Contains(js, `"ignored":"no authority`) {
		t.Errorf("JSON output lacks the ignored reason: %s", js)
	}
}

func TestValidate_Files(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		args     []string
		files    bool
		wantCode int
	}{
		{name: "valid document exits 0 without a resolver", data: goToolchain, args: []string{"--files", "x"}, files: true, wantCode: cli.ExitOK},
		{name: "invalid document", data: "kind: Nope\n", args: []string{"--files", "x"}, files: true, wantCode: cli.ExitInvalid},
		{name: "produced kind is checked, not loaded", data: "apiVersion: numinance.dev/v1\nkind: ContextBundle\nmetadata: {name: x}\n", args: []string{"--files", "x"}, files: true, wantCode: cli.ExitInvalid},
		{name: "produced kind is invalid without --files", data: "apiVersion: numinance.dev/v1\nkind: ContextBundle\n", args: []string{"x"}, wantCode: cli.ExitInvalid},
		{name: "build without a files source", data: goToolchain, args: []string{"--files", "x"}, wantCode: cli.ExitUsage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := oneDoc(tc.data)
			v := cli.Validate{Source: oneDoc(goToolchain)}
			if tc.files {
				v.Files = src
			} else {
				v.Source = src
			}
			if code := v.Run(context.Background(), tc.args); code != tc.wantCode {
				t.Errorf("Run(%v) = %d, want %d", tc.args, code, tc.wantCode)
			}
			if tc.files && !src.got.Files {
				t.Errorf("Files source got Request %+v, want Files set", src.got)
			}
		})
	}
}

func TestValidate_Files_ProducedKindPrintsNoEmptyName(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "examples", "ContextBundle", "implement-stage.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	v := cli.Validate{Files: oneDoc(string(b)), Stdout: &out}
	if code := v.Run(context.Background(), []string{"--files", "x"}); code != cli.ExitOK {
		t.Fatalf("Run = %d, want 0 (%q)", code, out.String())
	}
	if got := out.String(); !strings.Contains(got, "ContextBundle\n") || strings.Contains(got, `""`) {
		t.Errorf("output %q, want the kind and no empty name", got)
	}
}

func TestValidate_AlphaKinds_OnlyInFilesMode(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "examples", "EvalSet", "core-replay.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	v := cli.Validate{Files: oneDoc(string(b)), Source: oneDoc(string(b)), Stdout: &out}
	if code := v.Run(context.Background(), []string{"--files", "x"}); code != cli.ExitOK || !strings.Contains(out.String(), "EvalSet") {
		t.Errorf("Run(--files) = %d (%q), want 0 and the kind", code, out.String())
	}
	out.Reset()
	if code := v.Run(context.Background(), []string{"x"}); code != cli.ExitInvalid {
		t.Errorf("Run(repository) = %d (%q), want %d: an alpha kind is not configuration", code, out.String(), cli.ExitInvalid)
	}
}

func TestValidate_ReportsDeprecatedFieldsWithoutFailing(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "schemas", "examples", "Lockfile", "legacy-knowledge-tools.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "text", args: []string{"--files", "x"}, want: `DEPRECATED "spec.knowledge_tools" since 1.3.0, replaced by "numinance.kinds.v1.LockfileSpec.pinned_tools"`},
		{name: "json", args: []string{"--files", "--json", "x"}, want: `"reason":"DEPRECATED","path":"spec.knowledge_tools","deprecated_in":"1.3.0"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			v := cli.Validate{Files: oneDoc(string(b)), Stdout: &out}
			if code := v.Run(context.Background(), tt.args); code != cli.ExitOK {
				t.Fatalf("Run(%v) = %d, want 0: a deprecated field warns and does not fail (%q)", tt.args, code, out.String())
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Errorf("output %q, want it to contain %q", out.String(), tt.want)
			}
		})
	}
}
