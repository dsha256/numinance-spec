package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

// DefaultTimeout bounds one "numi validate" run.
const DefaultTimeout = 60 * time.Second

// Request names what to validate. RepoDir and OrgDir are paths the caller
// typed; a Source or Resolver treats them as untrusted.
type Request struct {
	// RepoDir is the repository; its repo and component trees are validated.
	RepoDir string
	// OrgDir is the optional organization policy repository.
	OrgDir string
	// Rev is the revision to read; empty means the Source's default (HEAD for a
	// repository, the working tree for the open build).
	Rev string
	// Component is the repository-relative root of the component; empty means no component tree.
	Component string
	// Files means RepoDir is a file or a directory of example kind documents,
	// not a repository: every YAML file below it is a document, no resolver
	// runs, and a pass exits 0. It excludes Rev, OrgDir and Component.
	Files bool
}

// Document is one kind document and where it was read from.
type Document struct {
	// Scope is the tree it was read from: "org", "repo" or "component".
	Scope string
	// Path is the tree path of the file, relative to its repository root.
	Path string
	// Data is the raw file, at most kindyaml.MaxBytes+1 bytes.
	Data []byte
}

// Snapshot is the documents of the trees a Request names.
type Snapshot struct {
	// Revision says what was read, for example a commit id or "working tree".
	Revision string
	// Documents are in a stable order.
	Documents []Document
}

// Source reads the kind documents of a Request. It is the only code that
// touches the repository, so it carries the path, size and count limits.
type Source interface {
	Snapshot(ctx context.Context, req Request) (Snapshot, error)
}

// FieldSource says which document set a resolved field.
type FieldSource struct {
	// Field is the dotted name of the resolved field, for example "autonomy.default_level".
	Field string
	// Scope is the tree of the document that set it: "org", "repo" or "component".
	Scope string
	// Path is the tree path of that document.
	Path string
	// Ignored is empty when the value took effect; otherwise it is the fixed
	// reason the document's value did not reach the effective profile.
	Ignored string
}

// Result is a successful resolution. It carries no field values, so a secret
// in a document cannot reach the output.
type Result struct {
	// Digest is the profile digest, for example "sha256:...".
	Digest     string
	Provenance []FieldSource
}

// Resolver resolves the profile of a Request. The engine build implements it;
// the open build has none. A floor violation is an error of KindInvalid.
//
// snap is the Snapshot whose documents passed the schema check. A Resolver
// resolves exactly those documents and does not read the Request's trees
// again, so what was checked is what is resolved.
type Resolver interface {
	Resolve(ctx context.Context, req Request, snap Snapshot) (Result, error)
}

// Validate is the "numi validate" command. Source is required. Without a
// Resolver it validates schemas only and exits ExitUnavailable, so a CI that
// uses the open build cannot take a schema-only pass for a full one.
type Validate struct {
	Source   Source
	Resolver Resolver
	// Files reads the documents for --files; nil means the build has no
	// example validator and --files is a usage error.
	Files Source
	// ErrorKind classifies errors from Source and Resolver; nil means DefaultErrorKind.
	ErrorKind func(error) Kind
	// Stdout and Stderr receive the report and the errors; nil discards.
	Stdout io.Writer
	Stderr io.Writer
	// Timeout bounds the run; zero means DefaultTimeout.
	Timeout time.Duration
}

type docReport struct {
	Scope  string `json:"scope"`
	Path   string `json:"path"`
	Kind   string `json:"kind,omitempty"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type provReport struct {
	Field string `json:"field"`
	Scope string `json:"scope"`
	Path  string `json:"path"`
	// Ignored is the reason the value did not take effect; omitted when it did.
	Ignored string `json:"ignored,omitempty"`
}

type report struct {
	Status     string       `json:"status"`
	Revision   string       `json:"revision,omitempty"`
	Documents  []docReport  `json:"documents,omitempty"`
	Digest     string       `json:"digest,omitempty"`
	Provenance []provReport `json:"provenance,omitempty"`
	Error      string       `json:"error,omitempty"`
}

// proceed is parse's code when the command should run.
const proceed = -1

const usageText = `usage: numi validate [--rev REV] [--org DIR] [--component ROOT] [--json] [path]
       numi validate --files [--json] path
  --rev REV         revision to read (default: the source's own; the open build reads the working tree and refuses any --rev)
  --org DIR         organization policy repository
  --component ROOT  repository-relative root of the component
  --files           path is a YAML file or a directory of example kind documents; schema check only (not with --rev, --org or --component)
  --json            print JSON
`

// Run parses args (the arguments after "validate"), runs the command and
// returns the exit code. Flags may come before or after the path. Nil Stdout
// and Stderr discard.
func (v Validate) Run(ctx context.Context, args []string) int {
	if v.Stdout == nil {
		v.Stdout = io.Discard
	}
	if v.Stderr == nil {
		v.Stderr = io.Discard
	}
	req, asJSON, code := v.parse(args)
	if code != proceed {
		return code
	}
	timeout := v.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rep := report{Status: "ok"}
	code = v.guarded(ctx, req, &rep)
	v.print(rep, asJSON)
	return code
}

func (v Validate) parse(args []string) (req Request, asJSON bool, code int) {
	fs := flag.NewFlagSet("numi validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // flag errors quote argv; print them through clean
	fs.StringVar(&req.Rev, "rev", "", "")
	fs.StringVar(&req.Component, "component", "", "")
	fs.StringVar(&req.OrgDir, "org", "", "")
	fs.BoolVar(&asJSON, "json", false, "")
	fs.BoolVar(&req.Files, "files", false, "")
	flagArgs, paths := splitArgs(args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(v.Stdout, usageText)
			return req, false, ExitOK
		}
		_, _ = fmt.Fprintf(v.Stderr, "numi validate: %s\n%s", clean(err.Error(), maxMessageRunes), usageText)
		return req, false, ExitUsage
	}
	if len(paths) > 1 {
		_, _ = fmt.Fprintf(v.Stderr, "numi validate: at most one path\n%s", usageText)
		return req, false, ExitUsage
	}
	if req.Files {
		switch {
		case v.Files == nil:
			_, _ = fmt.Fprintf(v.Stderr, "numi validate: --files is not available in this build\n%s", usageText)
			return req, false, ExitUsage
		case req.Rev != "" || req.OrgDir != "" || req.Component != "":
			_, _ = fmt.Fprintf(v.Stderr, "numi validate: --files excludes --rev, --org and --component\n%s", usageText)
			return req, false, ExitUsage
		case len(paths) != 1:
			_, _ = fmt.Fprintf(v.Stderr, "numi validate: --files needs exactly one path\n%s", usageText)
			return req, false, ExitUsage
		}
	}
	req.RepoDir = "."
	if len(paths) == 1 {
		req.RepoDir = paths[0]
	}
	return req, asJSON, proceed
}

// takesValue reports whether the named flag takes its value as the next argument.
func takesValue(name string) bool { return name == "rev" || name == "component" || name == "org" }

// splitArgs separates flags (with their values) from paths, so flags may come
// before or after a path. "--" ends flag parsing: everything after it is a
// path, even if it starts with "-" or equals "--json". A value-taking flag
// consumes the next argument whatever it looks like, including "--".
func splitArgs(args []string) (flagArgs, paths []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return flagArgs, append(paths, args[i+1:]...)
		case len(a) > 1 && a[0] == '-':
			flagArgs = append(flagArgs, a)
			name := strings.TrimLeft(a, "-")
			if takesValue(name) && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:
			paths = append(paths, a)
		}
	}
	return flagArgs, paths
}

func (v Validate) classify(err error) Kind {
	if v.ErrorKind != nil {
		return v.ErrorKind(err)
	}
	return DefaultErrorKind(err)
}

func (v Validate) fail(rep *report, err error) int {
	k := v.classify(err)
	if k == KindNone {
		k = KindInternal // an error never exits 0, whatever the classifier says
	}
	if errors.Is(err, context.Canceled) {
		k = KindInternal // 7 means "schema pass only"; a cancel must not look like it
	}
	rep.Status = "error"
	rep.Error = clean(err.Error(), maxMessageRunes)
	return ExitCode(k)
}

// guarded turns a panic in an injected Source or Resolver into ExitInternal,
// not the runtime's exit status 2, which would read as a usage error.
func (v Validate) guarded(ctx context.Context, req Request, rep *report) (code int) {
	defer func() {
		if recover() != nil {
			rep.Status = "error"
			rep.Error = "internal error"
			code = ExitInternal
		}
	}()
	return v.run(ctx, req, rep)
}

func (v Validate) run(ctx context.Context, req Request, rep *report) int {
	if err := ctx.Err(); err != nil {
		return v.fail(rep, err)
	}
	src := v.Source
	if req.Files {
		src = v.Files
	}
	if src == nil {
		return v.fail(rep, errors.New("numi: no document source"))
	}
	snap, err := src.Snapshot(ctx, req)
	if err != nil {
		return v.fail(rep, err)
	}
	rep.Revision = clean(snap.Revision, maxFieldRunes)
	// The example documents of an alpha kind (EvalSet) are validated with the rest; a repository tree never loads one.
	var opts []kindyaml.Option
	if req.Files {
		opts = append(opts, kindyaml.WithAlphaKinds())
	}
	loader, err := kindyaml.NewLoader(opts...)
	if err != nil {
		return v.fail(rep, err)
	}
	invalid := false
	for _, d := range snap.Documents {
		r := docReport{Scope: clean(d.Scope, maxFieldRunes), Path: clean(d.Path, maxFieldRunes), Status: "ok"}
		m, err := loader.Decode(d.Data)
		if err != nil && req.Files && errors.Is(err, kindyaml.ErrProduced) {
			// A produced kind has examples but is never configuration: check
			// it against its own strict schema, not as a loaded kind.
			m, err = loader.CheckProduced(d.Data)
		}
		switch {
		case err != nil:
			invalid = true
			r.Status = "invalid"
			r.Error = clean(err.Error(), maxMessageRunes)
		default:
			kind, name := identify(m)
			r.Kind, r.Name = kind, clean(name, maxFieldRunes)
		}
		rep.Documents = append(rep.Documents, r)
	}
	switch {
	case invalid:
		rep.Status = "invalid"
		return ExitInvalid
	case len(snap.Documents) == 0:
		rep.Status = "error"
		rep.Error = "no kind documents found"
		return ExitNotFound
	case ctx.Err() != nil:
		return v.fail(rep, ctx.Err())
	case req.Files:
		return ExitOK
	case v.Resolver == nil:
		rep.Status = "unavailable"
		rep.Error = "profile resolution needs the Numinance engine build"
		return ExitUnavailable
	}
	res, err := v.Resolver.Resolve(ctx, req, snap)
	if err != nil {
		return v.fail(rep, err)
	}
	rep.Digest = clean(res.Digest, maxFieldRunes)
	for _, p := range res.Provenance {
		rep.Provenance = append(rep.Provenance, provReport{
			Field: clean(p.Field, maxFieldRunes),
			Scope: clean(p.Scope, maxFieldRunes),
			Path:  clean(p.Path, maxFieldRunes),

			Ignored: clean(p.Ignored, maxFieldRunes),
		})
	}
	return ExitOK
}

// identify returns the kind and metadata.name of a decoded kind message.
func identify(m proto.Message) (kind, name string) {
	r := m.ProtoReflect()
	kind = string(r.Descriptor().Name())
	md := r.Descriptor().Fields().ByName("metadata")
	if md == nil || !r.Has(md) {
		return kind, ""
	}
	meta := r.Get(md).Message()
	if nf := meta.Descriptor().Fields().ByName("name"); nf != nil {
		name = meta.Get(nf).String()
	}
	return kind, name
}

func (v Validate) print(rep report, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(v.Stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(rep)
		return
	}
	w := v.Stdout
	if rep.Revision != "" {
		_, _ = fmt.Fprintf(w, "revision: \"%s\"\n", rep.Revision)
	}
	for _, d := range rep.Documents {
		if d.Status == "ok" {
			if d.Name == "" {
				_, _ = fmt.Fprintf(w, "ok       %s \"%s\" %s\n", d.Scope, d.Path, d.Kind)
				continue
			}
			_, _ = fmt.Fprintf(w, "ok       %s \"%s\" %s \"%s\"\n", d.Scope, d.Path, d.Kind, d.Name)
			continue
		}
		_, _ = fmt.Fprintf(w, "invalid  %s \"%s\": %s\n", d.Scope, d.Path, d.Error)
	}
	if rep.Digest != "" {
		_, _ = fmt.Fprintf(w, "digest: \"%s\"\n", rep.Digest)
		for _, p := range rep.Provenance {
			if p.Ignored != "" {
				_, _ = fmt.Fprintf(w, "  \"%s\" <- %s \"%s\" (ignored: \"%s\")\n", p.Field, p.Scope, p.Path, p.Ignored)
				continue
			}
			_, _ = fmt.Fprintf(w, "  \"%s\" <- %s \"%s\"\n", p.Field, p.Scope, p.Path)
		}
	}
	if rep.Error != "" {
		_, _ = fmt.Fprintf(v.Stderr, "numi validate: %s\n", rep.Error)
	}
}
