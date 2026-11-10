package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// discoverTimeout bounds one "numi discover" run.
const discoverTimeout = 30 * time.Second

const (
	// discoverReportFile is the report of a draft; like DISCOVERY.md of init it is outside policy/ and carries no authority.
	discoverReportFile = ".numinance/DISCOVER.md"
	// discoverPRFile is the proposed pull request text, beside the draft tree.
	discoverPRFile = "PULL_REQUEST.md"
)

const discoverUsage = `usage: numi discover --out DIR [--force] [path]
numi discover reads the enforcement artifacts of a repository (files only, nothing is executed, no git, gh or network command),
drafts the kinds numi init writes into DIR with a confidence annotation on each drafted value, and self-tests the draft.
  --out DIR   directory that receives the draft: DIR/.numinance/..., DIR/.numinance/DISCOVER.md and DIR/PULL_REQUEST.md
              (created, mode 0700, if missing; must be outside the repository)
  --force     overwrite the files discover writes in an existing DIR/.numinance (other files are never touched)
path is the repository to read (default .). It is read only; nothing is written to it.
The self-test decodes every drafted document with the kind loader, re-reads the repository and requires the same decisions,
and records the HEAD commit found in .git. A failed self-test writes nothing.
Opening the pull request is an operator step that is not built: discover prints the commands to run and runs none of them.
Exit codes: 0 ok, 2 usage, 3 invalid, 4 not found, 5 conflict, 1 internal.
`

// discoverOptions is the validated command line of "numi discover".
type discoverOptions struct {
	Path, Out string
	Force     bool
}

// discoverHooks are test seams of one run; the zero value does nothing. They let a test make the self-test fail
// without a package-level variable.
type discoverHooks struct {
	// mutateDraft may change the drafted files before the self-test.
	mutateDraft func([]outFile) []outFile
	// beforeReread runs just before the repository is read the second time.
	beforeReread func()
}

// RunDiscover runs "numi discover" with the arguments after "discover" and returns the exit code. It reads the
// repository as untrusted data exactly as "numi init" does (inspectRepo), drafts the same five kinds with a
// confidence annotation under extensions, self-tests the draft in process and writes it, a report and the text of a
// pull request into --out. It never opens the pull request: that is an operator step, and discover prints it.
func RunDiscover(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runDiscoverCommand(ctx, args, stdout, stderr, discoverHooks{})
}

func runDiscoverCommand(ctx context.Context, args []string, stdout, stderr io.Writer, hooks discoverHooks) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	ctx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()

	var opts discoverOptions
	fs := flag.NewFlagSet("numi discover", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.Out, "out", "", "")
	fs.BoolVar(&opts.Force, "force", false, "")
	usage := func(format string, a ...any) int {
		_, _ = fmt.Fprintf(stderr, "numi discover: %s\n%s", fmt.Sprintf(format, a...), discoverUsage)
		return ExitUsage
	}
	flagArgs, paths := splitArgs(args, func(name string) bool { return name == "out" })
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, discoverUsage)
			return ExitOK
		}
		return usage("%s", clean(err.Error(), maxMessageRunes))
	}
	if len(paths) > 1 {
		return usage("at most one path")
	}
	if opts.Out == "" {
		return usage("--out is required")
	}
	opts.Path = "."
	if len(paths) == 1 {
		opts.Path = paths[0]
	}
	report, err := runDiscover(ctx, opts, hooks)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "numi discover: %s\n", printable(err))
		return ExitCode(DefaultErrorKind(err))
	}
	_, _ = io.WriteString(stdout, report)
	return ExitOK
}

// runDiscover reads, drafts, self-tests and writes, and returns the report for stdout.
func runDiscover(ctx context.Context, opts discoverOptions, hooks discoverHooks) (string, error) {
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
	defer func() { _ = repoRoot.Close() }()
	out, err := checkOutDir(opts.Path, opts.Out)
	if err != nil {
		return "", err
	}

	first, err := readForDiscover(ctx, repoRoot)
	if err != nil {
		return "", err
	}
	if len(first.in.Hint.Conflict) > 0 {
		return "", Errorf(KindConflict, "the repository names two package managers (%s); discover does not choose between them, resolve it in the repository or write .numinance with numi init --package-manager", strings.Join(first.in.Hint.Conflict, ", "))
	}
	initOpts := initOptions{}
	disc, err := decide(first.in, &initOpts)
	if err != nil {
		return "", err
	}
	facts := newDiscoverFacts(first.in, initOpts, disc)
	data := newInitData(initOpts)
	data.Branch, data.Owners = disc.Branch, disc.Owners
	files, err := planRepoTree(data)
	if err != nil {
		return "", Errorf(KindInternal, "%v", err)
	}
	// The README of the tree is init's; its text says what init wrote. A draft is annotated and reported by discover.
	files = discoverDocuments(files, facts)
	if hooks.mutateDraft != nil {
		files = hooks.mutateDraft(files)
	}

	st, err := selfTest(ctx, selfTestInput{Path: opts.Path, Files: files, First: first, Facts: facts, BeforeReread: hooks.beforeReread})
	if err != nil {
		return "", err
	}
	files = append(files,
		outFile{Path: discoverReportFile, Data: facts.reportMarkdown(st)},
		outFile{Path: discoverPRFile, Data: facts.pullRequestMarkdown(st)},
	)

	created, err := ensureOutDir(out.real)
	if err != nil {
		return "", err
	}
	tree, err := openTreeFor("discover", out.real, files, opts.Force)
	if err != nil {
		removeIfCreated(out.real, created)
		return "", err
	}
	defer tree.close()
	if err := differentDirectories(repoRoot, tree, opts.Path, out.shown); err != nil {
		removeIfCreated(out.real, created)
		return "", err
	}
	if err := tree.write(ctx); err != nil {
		removeIfCreated(out.real, created)
		return "", err
	}
	return facts.stdout(opts, out.shown, st, files), nil
}

// outDir is the validated --out: the spelling the caller gave, and the real path that was checked and is written to.
type outDir struct{ shown, real string }

// checkOutDir refuses an --out that is the repository or inside it under any spelling, and returns the real path to
// write to, so that a link cannot redirect it later. A ".." element is refused: the operating system resolves it after
// a link, so the directory checked would not be the directory used.
func checkOutDir(repoPath, outPath string) (outDir, error) {
	for _, el := range strings.FieldsFunc(outPath, func(r rune) bool { return r == '/' || r == os.PathSeparator }) {
		if el == ".." {
			return outDir{}, safeErrorf(KindInvalid, "--out %s holds a '..' element; give a path without one", quote(outPath))
		}
	}
	if err := outsideRepo(repoPath, outPath); err != nil {
		return outDir{}, err
	}
	abs, err := filepath.Abs(outPath)
	if err != nil {
		return outDir{}, safeErrorf(KindInvalid, "--out %s cannot be resolved", quote(outPath))
	}
	return outDir{shown: outPath, real: realPath(abs)}, nil
}

// ensureOutDir creates the directory with mode 0700 when it is missing and reports whether it did.
func ensureOutDir(dir string) (created bool, err error) {
	switch err := os.Mkdir(dir, 0o700); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrExist):
		return false, nil
	default:
		return false, safeErrorf(KindInvalid, "cannot create --out %s", quote(dir))
	}
}

// removeIfCreated removes the out directory this run created, when nothing is left in it.
func removeIfCreated(dir string, created bool) {
	if created {
		_ = os.Remove(dir)
	}
}

// differentDirectories checks, on the directories as opened, that the out tree is not the repository or inside it.
func differentDirectories(repoRoot *os.Root, tree *treeWriter, repoPath, out string) error {
	a, errA := repoRoot.Stat(".")
	b, errB := tree.root.Stat(".")
	if errA != nil || errB != nil {
		return Errorf(KindInternal, "cannot inspect the directories")
	}
	if os.SameFile(a, b) {
		return safeErrorf(KindInvalid, "--out %s is the repository or inside it; give a directory outside the repository", quote(out))
	}
	return notInside(repoPath, out)
}
