package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
)

// initTimeout bounds one "numi init" run.
const initTimeout = 30 * time.Second

const initUsage = `usage: numi init [flags] [path]
numi init inspects the repository (files only, nothing is executed) and writes .numinance/ from what it finds. Every
flag below overrides what it found; the report says what was found and why each value was chosen.
  --ci SYSTEM             CI system(s) to wire: github-actions, jenkins or both (default: detected from .github/workflows
                          and Jenkinsfile; else github-actions)
  --org-dir DIR           also write the org-scoped documents (stage registry, grants, floors) into this organization policy repository
  --package-manager NAME  uv, pip or poetry (default: detected from uv.lock, poetry.lock, pyproject.toml, requirements.txt; else uv)
  --python-version X.Y    Python version of the toolchain (default: .python-version, .tool-versions, requires-python; else 3.12)
  --verifiers LIST        comma-separated subset of unit-tests,lint,typecheck (default: the ones whose tool (pytest, ruff, mypy)
                          the repository shows; else all three)
  --language python       toolchain language; python is the only value (default python)
  --scm github            source-control provider; github is the only value (default github)
  --tickets file          tickets come from the ticket file the run is given (numi v1a run --ticket FILE); writes no ticketing binding
                          (default: no ticket binding; no ticketing provider exists yet)
  --force                 overwrite the files init writes in an existing .numinance (other files are never touched)
path is the repository (default .). --org-dir naming the repository by any spelling (dot, relative, symlink) is refused with exit 3.
Exit codes: 0 ok, 2 usage, 3 invalid, 4 not found, 5 conflict, 1 internal.
`

// initOptions is the validated command line of "numi init".
type initOptions struct {
	Path           string
	OrgDir         string
	CIGitHub       bool
	CIJenkins      bool
	PackageManager string
	PythonVersion  string
	Verifiers      []string
	Force          bool
	// Language, SCM and Tickets are the values of the closed-set flags; empty when the flag was absent.
	Language, SCM, Tickets string
	// TicketFile is true when --tickets file was given.
	TicketFile bool
}

// closedChoice is a flag whose accepted set has one member today. Absent, the flag behaves as that member (or, for
// tickets, as no binding) and the report says so with source default; given, any other value is a usage error. The
// vendor names are table data, not logic.
type closedChoice struct {
	Flag, Accepted, Reason string
}

// closedChoices is the one table of the closed sets.
func closedChoices() []closedChoice {
	return []closedChoice{
		{Flag: "language", Accepted: "python", Reason: "only python has a toolchain template"},
		{Flag: "scm", Accepted: "github", Reason: "only github has a provider"},
		{Flag: "tickets", Accepted: "file", Reason: "no ticketing provider exists yet; use file"},
	}
}

// RunInit runs "numi init" with the arguments after "init" and returns the exit code. It inspects the repository
// (inspectRepo: files only, bounded, nothing executed) and writes a .numinance for it: an SDLCProfile, a
// StageRegistry, a Playbook, a LanguageToolchain, an AutonomyPolicy of PR only and the adapter bindings of the CI
// systems it found, plus DISCOVERY.md, the report of what it found and why it chose each value. A flag overrides what
// was found. The code names no CI vendor: vendor names are template data. It never overwrites without --force, never
// follows a link and writes through os.Root.
func RunInit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	ctx, cancel := context.WithTimeout(ctx, initTimeout)
	defer cancel()

	opts, code := parseInit(args, stdout, stderr)
	if code != proceed {
		return code
	}
	report, err := runInit(ctx, opts)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "numi init: %s\n", printable(err))
		return ExitCode(DefaultErrorKind(err))
	}
	_, _ = io.WriteString(stdout, report)
	return ExitOK
}

func parseInit(args []string, stdout, stderr io.Writer) (initOptions, int) {
	var (
		opts     initOptions
		ci       string
		verifier string
	)
	fs := flag.NewFlagSet("numi init", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // flag errors quote argv; print them through clean
	fs.StringVar(&ci, "ci", "", "")
	fs.StringVar(&opts.OrgDir, "org-dir", "", "")
	fs.StringVar(&opts.PackageManager, "package-manager", "", "")
	fs.StringVar(&opts.PythonVersion, "python-version", "", "")
	fs.StringVar(&verifier, "verifiers", "", "")
	fs.BoolVar(&opts.Force, "force", false, "")
	closed := map[string]*string{"language": &opts.Language, "scm": &opts.SCM, "tickets": &opts.Tickets}
	for _, c := range closedChoices() {
		fs.StringVar(closed[c.Flag], c.Flag, "", "")
	}
	flagArgs, paths := splitArgs(args, func(name string) bool {
		return slices.Contains([]string{"ci", "org-dir", "package-manager", "python-version", "verifiers", "language", "scm", "tickets"}, name)
	})
	usage := func(format string, a ...any) (initOptions, int) {
		_, _ = fmt.Fprintf(stderr, "numi init: %s\n%s", fmt.Sprintf(format, a...), initUsage)
		return opts, ExitUsage
	}
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, initUsage)
			return opts, ExitOK
		}
		return usage("%s", clean(err.Error(), maxMessageRunes))
	}
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	for _, c := range closedChoices() {
		if v := *closed[c.Flag]; given[c.Flag] && v != c.Accepted {
			return usage("--%s %s: %s", c.Flag, clean(v, maxFieldRunes), c.Reason)
		}
	}
	if len(paths) > 1 {
		return usage("at most one path")
	}
	opts.Path = "."
	if len(paths) == 1 {
		opts.Path = paths[0]
	}
	switch ci {
	case "":
		if given["ci"] {
			return usage("--ci is empty: github-actions, jenkins or both")
		}
		// Not given: runInit detects the CI system from the repository.
	case ciGitHubActions:
		opts.CIGitHub = true
	case ciJenkins:
		opts.CIJenkins = true
	case ciBoth:
		opts.CIGitHub, opts.CIJenkins = true, true
	default:
		return usage("--ci %s is not github-actions, jenkins or both", quote(ci))
	}
	if opts.PackageManager != "" && !slices.Contains(packageManagers(), opts.PackageManager) {
		return usage("--package-manager %s is not one of %s", quote(opts.PackageManager), strings.Join(packageManagers(), ", "))
	}
	if opts.PythonVersion != "" && !pythonVersionRe.MatchString(opts.PythonVersion) {
		return usage("--python-version %s is not of the form 3.N", quote(opts.PythonVersion))
	}
	if verifier != "" {
		for name := range strings.SplitSeq(verifier, ",") {
			if !slices.ContainsFunc(pythonVerifiers(), func(v verifierSpec) bool { return v.Name == name }) {
				return usage("--verifiers has %s, want unit-tests, lint or typecheck", quote(name))
			}
			if slices.Contains(opts.Verifiers, name) {
				return usage("--verifiers repeats %s", quote(name))
			}
			opts.Verifiers = append(opts.Verifiers, name)
		}
	}
	// Whether --org-dir is the repository is decided on the opened directories (runInit), not on the spelling.
	return opts, proceed
}

// closedChoiceOf is the choice of a closed-set flag: the flag value, or def with source default when it was absent.
func closedChoiceOf(v, def string) choice {
	if v == "" {
		return choice{Value: def, Source: "default"}
	}
	return choice{Value: v, Source: "flag"}
}

// runInit detects, plans, checks and writes, and returns the report for stdout.
func runInit(ctx context.Context, opts initOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	repoRoot, err := os.OpenRoot(opts.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", safeErrorf(KindNotFound, "directory %s not found", quote(opts.Path))
	case err != nil:
		return "", safeErrorf(KindInvalid, "%s is not a directory that can be opened", quote(opts.Path))
	}
	in, err := inspectRepo(ctx, repoRoot)
	_ = repoRoot.Close()
	if err != nil {
		return "", err
	}
	disc, err := decide(in, &opts)
	if err != nil {
		return "", err
	}

	data := newInitData(opts)
	data.Branch, data.Owners = disc.Branch, disc.Owners
	repoFiles, err := planRepoTree(data)
	if err != nil {
		return "", Errorf(KindInternal, "%v", err)
	}
	repoFiles = append(repoFiles, outFile{Path: discoveryFile, Data: disc.markdown()})
	repo, err := openTree(opts.Path, repoFiles, opts.Force)
	if err != nil {
		return "", err
	}
	defer repo.close()
	var org *treeWriter
	if opts.OrgDir != "" {
		orgFiles, err := planOrgTree(data)
		if err != nil {
			return "", Errorf(KindInternal, "%v", err)
		}
		if org, err = openTree(opts.OrgDir, orgFiles, opts.Force); err != nil {
			return "", err
		}
		defer org.close()
		if err := org.distinctFrom(repo); err != nil {
			return "", err
		}
	}
	if err := repo.write(ctx); err != nil {
		return "", err
	}
	if org != nil {
		if err := org.write(ctx); err != nil {
			repo.undo()
			return "", err
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "numi init: repository \"%s\"\n", clean(opts.Path, maxFieldRunes))
	foundLines, choseLines, noteLines := disc.stdoutLines()
	b.WriteString(foundLines + choseLines)
	for _, f := range repoFiles {
		fmt.Fprintf(&b, "wrote \"%s\"\n", f.Path)
	}
	if org != nil {
		fmt.Fprintf(&b, "organization policy repository \"%s\"\n", clean(opts.OrgDir, maxFieldRunes))
		for _, f := range org.files {
			fmt.Fprintf(&b, "wrote \"%s\"\n", f.Path)
		}
	}
	b.WriteString(noteLines)
	b.WriteString("autonomy: PR only (ladder level AUTONOMY_LEVEL_PR, no merge)\n")
	if data.Jenkins {
		b.WriteString("note: the Jenkins provider dev.numinance.ciexecutor-jenkins exists in replay mode only (a live run is owed and it is not yet in the release bundle); the Jenkins binding names it\n")
	}
	if data.Both {
		// Say only what was written: the repository tree holds the inert document, the org tree holds a grant.
		where := "the Jenkins binding is in .numinance/ci-jenkins.yaml and is ignored"
		if org != nil {
			where = "the Jenkins binding is an org grant in .numinance/policy/ci-jenkins.yaml of the organization policy repository and is not bound: the repository profile binds github-actions, and rebinding it to ciexecutor-jenkins is your explicit edit that validate does not check"
		}
		b.WriteString("note: one ciexecutor binding takes effect per scope: github-actions; " + where + "\n")
	}
	if data.HasTests() {
		b.WriteString("note: unit-tests (pytest) runs candidate code and is UNAVAILABLE in the live verifier provider until its nested sandbox exists\n")
	}
	if org == nil {
		b.WriteString("note: without --org-dir the stage registry is not selected and floors are the engine's shipped floors (PR; HIGH at SUGGEST)\n")
	}
	fmt.Fprintf(&b, "next: numi validate \"%s\"\n", clean(opts.Path, maxFieldRunes))
	return b.String(), nil
}
