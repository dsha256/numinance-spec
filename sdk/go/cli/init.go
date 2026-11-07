package cli

import (
	"cmp"
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

const initUsage = `usage: numi init --ci github-actions|jenkins|both [flags] [path]
  --ci SYSTEM             CI system(s) to wire: github-actions, jenkins or both (required)
  --org-dir DIR           also write the org-scoped documents (stage registry, grants, floors) into this organization policy repository
  --package-manager NAME  uv, pip or poetry (default: detected from uv.lock, poetry.lock, pyproject.toml, requirements.txt; else uv)
  --python-version X.Y    Python version of the toolchain (default: .python-version, else 3.12)
  --verifiers LIST        comma-separated subset of unit-tests,lint,typecheck (default: all three)
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
}

// RunInit runs "numi init" with the arguments after "init" and returns the exit code. It writes a .numinance template
// for a Python repository: an SDLCProfile, a StageRegistry, a Playbook, a LanguageToolchain, an AutonomyPolicy of PR
// only and the adapter bindings of the chosen CI systems. The documents are template data of the first onboarding; the
// code names no CI vendor. It never overwrites without --force, never follows a link and writes through os.Root.
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
	flagArgs, paths := splitArgs(args, func(name string) bool {
		return slices.Contains([]string{"ci", "org-dir", "package-manager", "python-version", "verifiers"}, name)
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
	if len(paths) > 1 {
		return usage("at most one path")
	}
	opts.Path = "."
	if len(paths) == 1 {
		opts.Path = paths[0]
	}
	switch ci {
	case "":
		return usage("--ci is required: github-actions, jenkins or both")
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
	h, err := detectHints(repoRoot)
	_ = repoRoot.Close()
	if err != nil {
		return "", err
	}
	if len(h.Conflict) > 0 && opts.PackageManager == "" {
		return "", Errorf(KindUsage, "conflicting package manager hints (%s); pass --package-manager", strings.Join(h.Conflict, ", "))
	}

	manager := choice{Value: opts.PackageManager, Source: "flag"}
	if manager.Value == "" {
		manager = choice{Value: cmp.Or(h.PackageManager, defaultPackageManager), Source: "detected from " + h.Source}
		if h.PackageManager == "" {
			manager.Source = "default, no hint in the directory"
		}
	}
	version := choice{Value: opts.PythonVersion, Source: "flag"}
	if version.Value == "" {
		version = choice{Value: cmp.Or(h.PythonVersion, defaultPythonVersion), Source: "detected from " + h.VersionSource}
		if h.PythonVersion == "" {
			version.Source = "default"
		}
	}
	verifiers := choice{Value: strings.Join(opts.Verifiers, " "), Source: "flag"}
	if len(opts.Verifiers) == 0 {
		all := pythonVerifiers()
		names := make([]string, len(all))
		for i, v := range all {
			names[i] = v.Name
		}
		opts.Verifiers = names
		verifiers = choice{Value: strings.Join(names, " "), Source: "default"}
	}
	opts.PackageManager, opts.PythonVersion = manager.Value, version.Value

	data := newInitData(opts)
	repoFiles, err := planRepoTree(data)
	if err != nil {
		return "", Errorf(KindInternal, "%v", err)
	}
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
	fmt.Fprintf(&b, "chose: package manager %s\n", manager)
	fmt.Fprintf(&b, "chose: python %s\n", version)
	fmt.Fprintf(&b, "chose: verifiers %s\n", verifiers)
	fmt.Fprintf(&b, "chose: ci %s (flag)\n", data.CIText())
	for _, f := range repoFiles {
		fmt.Fprintf(&b, "wrote \"%s\"\n", f.Path)
	}
	if org != nil {
		fmt.Fprintf(&b, "organization policy repository \"%s\"\n", clean(opts.OrgDir, maxFieldRunes))
		for _, f := range org.files {
			fmt.Fprintf(&b, "wrote \"%s\"\n", f.Path)
		}
	}
	b.WriteString("autonomy: PR only (ladder level AUTONOMY_LEVEL_PR, no merge)\n")
	if data.Jenkins {
		b.WriteString("note: no Jenkins provider exists yet; the Jenkins binding names a placeholder provider id\n")
	}
	if data.Both {
		// Say only what was written: the repository tree holds the inert document, the org tree holds a grant.
		where := "the Jenkins binding is in .numinance/ci-jenkins.yaml and is ignored"
		if org != nil {
			where = "the Jenkins binding is an org grant in .numinance/policy/ci-jenkins.yaml of the organization policy repository and is not bound: the repository profile binds github-actions, and rebinding it to the placeholder is your explicit edit that validate does not check"
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
