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

// suggestTimeout bounds one "numi suggest" run.
const suggestTimeout = 30 * time.Second

const suggestUsage = `usage: numi suggest --out DIR [path]
  --out DIR   directory that receives the ticket files (created, mode 0700, if missing; must be outside the repository; an existing ticket file is never overwritten)
path is the Python repository to read (default .). It is read only; nothing is written to it.
Proposes up to three bounded tickets, each in the format "numi v1a run --ticket FILE --path P" consumes, with the exact --path to use.
It is a deterministic local scan (public functions that no test file mentions, missing type hints, missing docstrings, TODO and FIXME comments).
No model is called, no repository code is run, no network is used. It is not an Engine run and makes no policy decision.
Exit codes: 0 ok, 2 usage, 3 invalid, 4 not found, 5 conflict, 1 internal.
`

// RunSuggest runs "numi suggest" with the arguments after "suggest" and returns the exit code. It reads a Python
// repository through bounded readers and writes at most three ticket files into --out; see suggestUsage.
func RunSuggest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	ctx, cancel := context.WithTimeout(ctx, suggestTimeout)
	defer cancel()

	fs := flag.NewFlagSet("numi suggest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var out string
	fs.StringVar(&out, "out", "", "")
	usage := func(format string, a ...any) int {
		_, _ = fmt.Fprintf(stderr, "numi suggest: %s\n%s", fmt.Sprintf(format, a...), suggestUsage)
		return ExitUsage
	}
	flagArgs, paths := splitArgs(args, func(name string) bool { return name == "out" })
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, suggestUsage)
			return ExitOK
		}
		return usage("%s", clean(err.Error(), maxMessageRunes))
	}
	if len(paths) > 1 {
		return usage("at most one path")
	}
	if out == "" {
		return usage("--out is required")
	}
	repo := "."
	if len(paths) == 1 {
		repo = paths[0]
	}
	report, err := runSuggest(ctx, repo, out)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "numi suggest: %s\n", printable(err))
		return ExitCode(DefaultErrorKind(err))
	}
	_, _ = io.WriteString(stdout, report)
	return ExitOK
}

func runSuggest(ctx context.Context, repoPath, outPath string) (string, error) {
	repoRoot, err := os.OpenRoot(repoPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", safeErrorf(KindNotFound, "directory %s not found", quote(repoPath))
	case err != nil:
		return "", safeErrorf(KindInvalid, "%s is not a directory that can be opened", quote(repoPath))
	}
	defer func() { _ = repoRoot.Close() }()
	// A ".." after a link is resolved by the operating system, not lexically: refuse it, so the directory checked is the directory used.
	for _, el := range strings.FieldsFunc(outPath, func(r rune) bool { return r == '/' || r == os.PathSeparator }) {
		if el == ".." {
			return "", safeErrorf(KindInvalid, "--out %s holds a '..' element; give a path without one", quote(outPath))
		}
	}
	if err := outsideRepo(repoPath, outPath); err != nil {
		return "", err
	}
	// What is written to is the real path that was checked, never a spelling that a link could redirect later.
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return "", safeErrorf(KindInvalid, "--out %s cannot be resolved", quote(outPath))
	}
	shownOut := outPath
	outPath = realPath(absOut)
	res, err := scan(ctx, repoRoot)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", Errorf(KindInternal, "cannot scan the repository")
	}
	picked := selectFindings(res)
	sugs := make([]suggestion, len(picked))
	for i, f := range picked {
		sugs[i] = renderSuggestion(f, i+1, res.TestDir)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "numi suggest: repository \"%s\"\n", clean(repoPath, maxFieldRunes))
	fmt.Fprintf(&b, "scanned: %d python files (%d source, %d test), %d bytes\n", res.Files, res.Sources, res.Tests, res.Bytes)
	if res.Truncated {
		b.WriteString("note: a scan bound was reached; the files read first (lexical order) were used\n")
	}
	if len(sugs) == 0 {
		b.WriteString("no suggestions: the scan found nothing it can turn into a bounded ticket\n")
		return b.String(), nil
	}
	written, err := writeSuggestions(repoPath, outPath, sugs)
	if err != nil {
		return "", err
	}
	for i := range written {
		written[i] = filepath.Join(shownOut, filepath.Base(written[i])) // shown as the caller spelled it
	}
	for i, s := range sugs {
		fmt.Fprintf(&b, "suggestion %d: %s\n", i+1, s.Kind)
		fmt.Fprintf(&b, "  ticket: \"%s\"\n", clean(written[i], maxMessageRunes))
		fmt.Fprintf(&b, "  run: numi v1a run --ticket \"%s\" --path \"%s\" (plus your usual flags)\n", clean(written[i], maxMessageRunes), s.Paths[0])
	}
	b.WriteString("note: a deterministic local scan, not an Engine run: no model, no policy decision; the repository was only read\n")
	b.WriteString("note: read each ticket before running it; the --path shown is the only file the change may touch\n")
	return b.String(), nil
}

// outsideRepo refuses an --out directory that is the repository or inside it, under any spelling or link, so a suggest
// run never writes into the repository.
func outsideRepo(repoPath, outPath string) error {
	repo, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		return safeErrorf(KindNotFound, "directory %s not found", quote(repoPath))
	}
	if repo, err = filepath.Abs(repo); err != nil {
		return Errorf(KindInternal, "cannot resolve the repository path")
	}
	abs, err := filepath.Abs(outPath)
	if err != nil {
		return safeErrorf(KindInvalid, "--out %s cannot be resolved", quote(outPath))
	}
	// The directory may not exist yet: resolve its parent and keep the last element.
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return safeErrorf(KindNotFound, "the parent of --out %s does not exist", quote(outPath))
	}
	resolved := filepath.Join(parent, filepath.Base(abs))
	if existing, err := filepath.EvalSymlinks(resolved); err == nil {
		resolved = existing
	}
	rel, err := filepath.Rel(repo, resolved)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return safeErrorf(KindInvalid, "--out %s is the repository or inside it; give a directory outside the repository", quote(outPath))
	}
	// A spelling that differs only in case (a case-insensitive file system), a bind or any alias passes the comparison above, so the
	// directories are compared by identity: no ancestor of --out may be the repository itself.
	return notInside(repoPath, outPath)
}

// notInside refuses out when it, or an existing ancestor of it, is the same file as repoPath.
func notInside(repoPath, out string) error {
	repoInfo, err := os.Stat(repoPath)
	if err != nil {
		return safeErrorf(KindNotFound, "directory %s not found", quote(repoPath))
	}
	abs, err := filepath.Abs(out)
	if err != nil {
		return safeErrorf(KindInvalid, "--out %s cannot be resolved", quote(out))
	}
	// Both the spelling and the real path (links resolved) are walked: a link to a subdirectory of the repository has ancestors
	// that are the repository only on the real path.
	for _, start := range []string{abs, realPath(abs)} {
		for dir := start; ; dir = filepath.Dir(dir) {
			if info, err := os.Stat(dir); err == nil && os.SameFile(info, repoInfo) {
				return safeErrorf(KindInvalid, "--out %s is the repository or inside it; give a directory outside the repository", quote(out))
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return nil
}

// realPath resolves the links of the longest existing prefix of p and keeps the rest as spelled.
func realPath(p string) string {
	rest := ""
	for dir := p; ; dir = filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		rest = filepath.Join(filepath.Base(dir), rest)
		if filepath.Dir(dir) == dir {
			return p
		}
	}
}

// writeSuggestions writes the ticket files into out (created with mode 0700 when missing) through os.Root, each mode 0600
// and never over an existing file, and returns the written paths. On a failure the files this run created are removed.
func writeSuggestions(repoPath, out string, sugs []suggestion) ([]string, error) {
	if err := os.Mkdir(out, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, safeErrorf(KindInvalid, "cannot create --out %s", quote(out))
	}
	root, err := os.OpenRoot(out)
	if err != nil {
		return nil, safeErrorf(KindInvalid, "--out %s is not a directory that can be opened", quote(out))
	}
	defer func() { _ = root.Close() }()
	// The directory was created or followed after the first check: the identity check runs again on what was opened.
	if err := notInside(repoPath, out); err != nil {
		return nil, err
	}
	var created []string
	undo := func() {
		for _, n := range created {
			_ = root.Remove(n)
		}
	}
	paths := make([]string, len(sugs))
	for i, s := range sugs {
		f, err := root.OpenFile(s.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			undo()
			if errors.Is(err, os.ErrExist) {
				return nil, safeErrorf(KindConflict, "%s already exists in --out; remove it or choose another directory", quote(s.Name))
			}
			return nil, safeErrorf(KindInternal, "cannot create %s in --out", quote(s.Name))
		}
		created = append(created, s.Name)
		_, werr := io.WriteString(f, s.Text)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			undo()
			return nil, safeErrorf(KindInternal, "cannot write %s in --out", quote(s.Name))
		}
		paths[i] = filepath.Join(out, s.Name)
	}
	return paths, nil
}
