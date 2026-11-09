package cli

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Bounds on what "numi init" reads while it inspects a repository. Inspection reads no more than these bytes and
// entries, whatever the repository holds; the report says when a bound cut something.
const (
	maxWalkEntries    = 5000
	maxWalkDepth      = 4
	maxDirEntries     = 2000
	maxWorkflowFiles  = 32
	maxRequirements   = 8
	maxCodeownersRule = 5000
)

// The shape of a name init may print or write into a document. A name outside it is counted and skipped, never quoted
// into a file: repository names are untrusted data.
var (
	safeNameRe    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
	safePatternRe = regexp.MustCompile(`^[A-Za-z0-9._*/-]{1,100}$`)
	branchRe      = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,100}$`)

	requiresPythonRe = regexp.MustCompile(`(?m)^\s*requires-python\s*=\s*["']([^"'\n]{1,100})["']`)
	poetryPythonRe   = regexp.MustCompile(`(?m)^\s*python\s*=\s*["']([^"'\n]{1,100})["']`)
	lowerBoundRe     = regexp.MustCompile(`(?:>=|~=|\^|==|~>|~)\s*(3\.\d{1,2})`)
	toolVersionsRe   = regexp.MustCompile(`(?m)^python\s+(3\.\d{1,2})(?:\.\d+)?\s*$`)
)

// repoReader reads the repository below an os.Root: regular files only, no link followed, each read cut at a limit.
// The first read error is kept in err and later reads return nothing, so a caller checks once at the end.
type repoReader struct {
	root *os.Root
	err  error
	// cut names the files that were larger than the limit they were read with.
	cut []string
}

// text returns at most limit bytes of the regular file name, or nil when it is absent, not regular or under a link.
func (r *repoReader) text(name string, limit int64) []byte {
	if r.err != nil || !r.plainPath(name) {
		return nil
	}
	f, err := r.root.Open(name)
	if err != nil {
		r.err = Errorf(KindInternal, "cannot read %s", name)
		return nil
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		r.err = Errorf(KindInternal, "cannot read %s", name)
		return nil
	}
	if int64(len(b)) > limit {
		b = b[:limit]
		r.cut = append(r.cut, name)
	}
	if b == nil {
		b = []byte{}
	}
	return b
}

// plainPath reports whether name is a regular file and no directory on the way to it is a link.
func (r *repoReader) plainPath(name string) bool {
	if r.err != nil {
		return false
	}
	dir := path.Dir(name)
	if dir != "." && !r.plainDir(dir) {
		return false
	}
	ok, err := regularFile(r.root, name)
	if err != nil {
		r.err = err
		return false
	}
	return ok
}

// plainDir reports whether name is a directory and neither it nor any parent is a link.
func (r *repoReader) plainDir(name string) bool {
	if r.err != nil {
		return false
	}
	if name == "." {
		return true
	}
	built := ""
	for part := range strings.SplitSeq(name, "/") {
		built = path.Join(built, part)
		fi, err := r.root.Lstat(built)
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}
		if err != nil {
			r.err = Errorf(KindInternal, "cannot inspect %s", built)
			return false
		}
		if !fi.IsDir() {
			return false
		}
	}
	return true
}

// list returns the entries of the directory name, sorted, at most limit of them; more is true when the directory held
// more. A link in the path or a missing directory lists nothing.
func (r *repoReader) list(name string, limit int) (entries []fs.DirEntry, more bool) {
	if !r.plainDir(name) {
		return nil, false
	}
	d, err := r.root.Open(name)
	if err != nil {
		r.err = Errorf(KindInternal, "cannot read %s", name)
		return nil, false
	}
	defer func() { _ = d.Close() }()
	entries, err = d.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = Errorf(KindInternal, "cannot read %s", name)
		return nil, false
	}
	if len(entries) > limit {
		entries, more = entries[:limit], true
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, more
}

// pythonVersionHint finds the Python version the repository asks for, in the order .python-version, .tool-versions,
// requires-python and the poetry python constraint. A lower bound is taken: it is the oldest version the code must run
// on. A value that is not a plain 3.N is not guessed at.
func pythonVersionHint(r *repoReader, pyproject []byte) (version, source string) {
	if b := r.text(".python-version", maxVersionBytes); b != nil {
		first, _, _ := strings.Cut(string(b), "\n")
		if m := dotPythonVersionRe.FindStringSubmatch(strings.TrimSpace(first)); m != nil {
			return m[1], ".python-version"
		}
	}
	if b := r.text(".tool-versions", maxHintBytes); b != nil {
		if m := toolVersionsRe.FindSubmatch(b); m != nil {
			return string(m[1]), ".tool-versions"
		}
	}
	if m := requiresPythonRe.FindSubmatch(pyproject); m != nil {
		if lb := lowerBoundRe.FindSubmatch(m[1]); lb != nil {
			return string(lb[1]), "pyproject.toml requires-python"
		}
	}
	if strings.Contains(string(pyproject), "[tool.poetry.dependencies]") {
		if m := poetryPythonRe.FindSubmatch(pyproject); m != nil {
			if lb := lowerBoundRe.FindSubmatch(m[1]); lb != nil {
				return string(lb[1]), "pyproject.toml [tool.poetry.dependencies] python"
			}
		}
	}
	return "", ""
}

// toolSpec is how one verifier's tool shows itself in a repository. The tables are data; the order of the checks in
// findTool is the strength of the evidence: a settings file, then a pyproject table, then a section of another file,
// then a dependency, then a use in a hook or CI file.
type toolSpec struct {
	Verifier string
	Tool     string
	// Files are settings files at the repository root or below it.
	Files []string
	// Hints are files that show the tool is used but do not configure it; they count after the sections below.
	Hints []string
	// Table is the name of the pyproject.toml table that configures the tool, for the report.
	Table string
	// PyprojectTable is the regexp of the pyproject.toml table header.
	PyprojectTable *regexp.Regexp
	// OtherFiles lists files with the regexp of the section that configures the tool in each.
	OtherFiles []otherFile
	// Word matches the tool name as a word in a dependency list, hook file or CI file.
	Word *regexp.Regexp
}

type otherFile struct {
	Name    string
	Section *regexp.Regexp
}

func toolSpecs() []toolSpec {
	return []toolSpec{
		{
			Verifier: "unit-tests", Tool: "pytest",
			Table:          "tool.pytest.ini_options",
			Files:          []string{"pytest.ini"},
			Hints:          []string{"conftest.py", "tests/conftest.py"},
			PyprojectTable: regexp.MustCompile(`(?m)^\[tool\.pytest[.\]]`),
			OtherFiles: []otherFile{
				{"setup.cfg", regexp.MustCompile(`(?m)^\[tool:pytest\]`)},
				{"tox.ini", regexp.MustCompile(`(?m)^\[pytest\]`)},
			},
			Word: toolWord("pytest"),
		},
		{
			Verifier: "lint", Tool: "ruff",
			Table:          "tool.ruff",
			Files:          []string{"ruff.toml", ".ruff.toml"},
			PyprojectTable: regexp.MustCompile(`(?m)^\[tool\.ruff[.\]]`),
			Word:           toolWord("ruff"),
		},
		{
			Verifier: "typecheck", Tool: "mypy",
			Table:          "tool.mypy",
			Files:          []string{"mypy.ini", ".mypy.ini"},
			PyprojectTable: regexp.MustCompile(`(?m)^\[tool\.mypy[.\]]`),
			OtherFiles:     []otherFile{{"setup.cfg", regexp.MustCompile(`(?m)^\[mypy\]`)}},
			Word:           toolWord("mypy"),
		},
	}
}

// toolWord matches name as a word: not inside a longer name, not as a dotted table segment.
func toolWord(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[^a-z0-9_.-])` + name + `(?:$|[^a-z0-9_])`)
}

// foundTool is a tool that the repository shows, and the file that shows it.
type foundTool struct {
	Tool, Where string
}

// langCount is the number of source files of one language found by the bounded walk.
type langCount struct {
	Name  string
	Files int
}

// extensionLanguages maps a file extension to a language name. Table data: it is how the report names what it saw.
func extensionLanguages() map[string]string {
	return map[string]string{
		".py": "python", ".pyi": "python",
		".go": "go", ".rs": "rust", ".rb": "ruby", ".php": "php", ".cs": "csharp", ".java": "java", ".kt": "kotlin",
		".ts": "typescript", ".tsx": "typescript", ".js": "javascript", ".jsx": "javascript", ".mjs": "javascript",
		".c": "c", ".h": "c", ".cc": "cpp", ".cpp": "cpp",
	}
}

// walkSkipDirs are directories whose content is not the project's own code.
func walkSkipDirs() map[string]bool {
	return map[string]bool{
		"vendor": true, "node_modules": true, "venv": true, "site-packages": true, "__pycache__": true, "third_party": true,
	}
}

// conventionFiles are files that state how a repository wants to be worked on. They are listed, never read: their
// text is untrusted and is not instructions to init.
func conventionFiles() []string {
	return []string{
		"AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md", ".editorconfig", ".pre-commit-config.yaml", "Makefile", "tox.ini", "noxfile.py",
		".github/PULL_REQUEST_TEMPLATE.md", ".github/pull_request_template.md",
	}
}

// unmappedManagers are files of a package manager or environment init has no toolchain value for.
func unmappedManagers() []string {
	return []string{"Pipfile", "pdm.lock", "environment.yml", "setup.py"}
}

// codeownersPaths are the places a CODEOWNERS file is read from, in the order the host reads them.
func codeownersPaths() []string {
	return []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}
}

// inspection is everything "numi init" learned by reading the repository, as data. It holds no unvetted name.
type inspection struct {
	Hint hint
	// Languages is sorted by file count, largest first.
	Languages []langCount
	// Workflows are the safe names of the workflow files below .github/workflows.
	Workflows []string
	// OddWorkflows counts workflow files whose names are outside the safe shape and were skipped.
	OddWorkflows int
	// Dispatch is true when a workflow declares workflow_dispatch.
	Dispatch    bool
	Jenkinsfile bool
	// Tools holds one finding per verifier name.
	Tools map[string]foundTool
	// DefaultBranch and BranchSource: the branch, and where it was read; BranchIsCurrent is true when only the checked
	// out branch was found, which is not the same as the default.
	DefaultBranch, BranchSource string
	BranchIsCurrent             bool
	// Codeowners is the path of the file read; Rules counts its rules; Covers is the last pattern that covers
	// .numinance/, empty when none does.
	Codeowners, Covers string
	Rules              int
	Conventions        []string
	Unmapped           []string
	// NestedProjects counts pyproject.toml files below the root; init reads the root one only.
	NestedProjects int
	// SkippedLinks and SkippedHidden count what the walk did not enter or count; SkippedDirs names vendor-like
	// directories it did not enter.
	SkippedLinks, SkippedHidden int
	SkippedDirs                 []string
	// Cut lists the bounds that cut something: a file larger than its read limit, a directory or walk over its limit.
	Cut []string
}

// inspectRepo reads the repository at root. It executes nothing, follows no link and reads at most the bounds above.
func inspectRepo(ctx context.Context, root *os.Root) (inspection, error) {
	r := &repoReader{root: root}
	var in inspection
	var err error
	if in.Hint, err = detectHints(r); err != nil {
		return inspection{}, err
	}
	pyproject := r.text("pyproject.toml", maxHintBytes)

	in.Jenkinsfile = r.plainPath("Jenkinsfile")
	entries, more := r.list(".github/workflows", maxWorkflowFiles)
	if more {
		in.Cut = append(in.Cut, "more than 32 workflow files; the first 32 by name were read")
	}
	var workflowPaths []string
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		if !safeNameRe.MatchString(name) {
			in.OddWorkflows++
			continue
		}
		in.Workflows = append(in.Workflows, name)
		workflowPaths = append(workflowPaths, ".github/workflows/"+name)
	}
	var requirements []string
	top, _ := r.list(".", maxDirEntries)
	for _, e := range top {
		n := e.Name()
		if e.Type().IsRegular() && strings.HasSuffix(n, ".txt") && strings.Contains(n, "requirements") && safeNameRe.MatchString(n) && len(requirements) < maxRequirements {
			requirements = append(requirements, n)
		}
	}
	// Sources are read in the order of their evidence: requirements, the hook file, Jenkinsfile, workflows.
	sources := slices.Concat(requirements, []string{".pre-commit-config.yaml", "Jenkinsfile"}, workflowPaths)
	texts := map[string][]byte{}
	for _, s := range sources {
		texts[s] = r.text(s, maxHintBytes)
		if texts[s] != nil && strings.HasPrefix(s, ".github/") && strings.Contains(string(texts[s]), "workflow_dispatch") {
			in.Dispatch = true
		}
	}
	in.Tools = map[string]foundTool{}
	for _, spec := range toolSpecs() {
		if where := findTool(r, spec, pyproject, texts, sources); where != "" {
			in.Tools[spec.Verifier] = foundTool{Tool: spec.Tool, Where: where}
		}
	}

	in.DefaultBranch, in.BranchSource, in.BranchIsCurrent = findBranch(r)
	findCodeowners(r, &in)
	for _, name := range conventionFiles() {
		if r.plainPath(name) {
			in.Conventions = append(in.Conventions, name)
		}
	}
	for _, name := range unmappedManagers() {
		if r.plainPath(name) {
			in.Unmapped = append(in.Unmapped, name)
		}
	}
	if err := walkRepo(ctx, r, &in); err != nil {
		return inspection{}, err
	}
	for _, f := range slices.Compact(slices.Sorted(slices.Values(r.cut))) {
		in.Cut = append(in.Cut, f+" is larger than 64 KiB; the first 64 KiB were read")
	}
	if r.err != nil {
		return inspection{}, r.err
	}
	return in, nil
}

// findTool returns the place that shows the tool, strongest evidence first, or "" when none does.
func findTool(r *repoReader, spec toolSpec, pyproject []byte, texts map[string][]byte, sources []string) string {
	for _, f := range spec.Files {
		if r.plainPath(f) {
			return f
		}
	}
	if spec.PyprojectTable.Match(pyproject) {
		return "pyproject.toml [" + spec.Table + "]"
	}
	for _, o := range spec.OtherFiles {
		if b := r.text(o.Name, maxHintBytes); b != nil && o.Section.Match(b) {
			return o.Name
		}
	}
	for _, f := range spec.Hints {
		if r.plainPath(f) {
			return f
		}
	}
	if spec.Word.Match(pyproject) {
		return "pyproject.toml (dependency)"
	}
	for _, s := range sources {
		if b := texts[s]; b != nil && spec.Word.Match(b) {
			return s
		}
	}
	return ""
}

// findBranch reads the default branch from the remote HEAD the clone recorded; failing that, the checked-out branch.
func findBranch(r *repoReader) (branch, source string, current bool) {
	for _, c := range []struct{ file, prefix string }{
		{".git/refs/remotes/origin/HEAD", "ref: refs/remotes/origin/"},
		{".git/HEAD", "ref: refs/heads/"},
	} {
		line, _, _ := strings.Cut(string(r.text(c.file, maxVersionBytes)), "\n")
		name, ok := strings.CutPrefix(strings.TrimSpace(line), c.prefix)
		if ok && branchRe.MatchString(name) {
			return name, c.file, c.file == ".git/HEAD"
		}
	}
	return "", "", false
}

// findCodeowners reads the first CODEOWNERS file and notes whether a rule covers .numinance/.
func findCodeowners(r *repoReader, in *inspection) {
	for _, p := range codeownersPaths() {
		b := r.text(p, maxHintBytes)
		if b == nil {
			continue
		}
		in.Codeowners = p
		// The last matching rule wins (CODEOWNERS semantics): a later rule that matches the policy directory with no owner removes
		// the ownership an earlier rule gave it.
		for line := range strings.Lines(string(b)) {
			fields := strings.Fields(line)
			if len(fields) < 1 || fields[0][0] == '#' || in.Rules >= maxCodeownersRule {
				continue
			}
			in.Rules++
			if !safePatternRe.MatchString(fields[0]) || !coversPolicyDir(fields[0]) {
				continue
			}
			if len(fields) < 2 {
				in.Covers = ""
				continue
			}
			in.Covers = fields[0]
		}
		return
	}
}

// coversPolicyDir reports whether a CODEOWNERS pattern gives an owner to .numinance/policy/: the whole tree, the .numinance
// directory or the policy directory, with or without a leading slash. A lookalike (.numinancex, .numinance-old/) or a narrower
// directory (.numinance/docs/) does not.
func coversPolicyDir(pattern string) bool {
	switch strings.TrimPrefix(pattern, "/") {
	case "*", "**", "**/*", ".numinance", ".numinance/", ".numinance/*", ".numinance/**", ".numinance/policy", ".numinance/policy/", ".numinance/policy/*", ".numinance/policy/**":
		return true
	}
	return false
}

// walkRepo counts source files by language below the root, skipping links, hidden entries and vendor-like directories.
func walkRepo(ctx context.Context, r *repoReader, in *inspection) error {
	counts := map[string]int{}
	skipNames, languages := walkSkipDirs(), extensionLanguages()
	skipDirs := map[string]bool{}
	visited := 0
	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, more := r.list(dir, maxDirEntries)
		if more {
			in.Cut = append(in.Cut, "a directory held more than 2000 entries; the first 2000 by name were read")
		}
		for _, e := range entries {
			if visited >= maxWalkEntries {
				return nil
			}
			name := e.Name()
			if dir == "." && name == configRoot {
				// What a previous run wrote is not the project's own content; leaving it out keeps a rerun's report equal.
				continue
			}
			visited++
			switch {
			case e.Type()&fs.ModeSymlink != 0:
				in.SkippedLinks++
			case strings.HasPrefix(name, "."):
				in.SkippedHidden++
			case e.IsDir() && skipNames[name]:
				skipDirs[name] = true
			case e.IsDir():
				if depth < maxWalkDepth {
					if err := walk(path.Join(dir, name), depth+1); err != nil {
						return err
					}
				}
			case e.Type().IsRegular():
				if name == "pyproject.toml" && dir != "." {
					in.NestedProjects++
				}
				if lang, ok := languages[strings.ToLower(path.Ext(name))]; ok {
					counts[lang]++
				}
			}
		}
		return nil
	}
	if err := walk(".", 0); err != nil {
		return err
	}
	if visited >= maxWalkEntries {
		in.Cut = append(in.Cut, "the walk stopped after 5000 entries")
	}
	for lang, n := range counts {
		in.Languages = append(in.Languages, langCount{Name: lang, Files: n})
	}
	slices.SortFunc(in.Languages, func(a, b langCount) int {
		if a.Files != b.Files {
			return b.Files - a.Files
		}
		return strings.Compare(a.Name, b.Name)
	})
	in.SkippedDirs = slices.Sorted(func(yield func(string) bool) {
		for k := range skipDirs {
			if !yield(k) {
				return
			}
		}
	})
	return r.err
}
