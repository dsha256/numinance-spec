package provider

import (
	"archive/tar"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Limits of an extracted candidate.
const (
	maxFiles    = 20000
	maxFileSize = 16 << 20
)

// ErrCandidate is the error of a candidate artifact that is not a tar archive of the shape the provider accepts.
var ErrCandidate = errors.New("the candidate artifact is not an acceptable tar archive")

// Extract unpacks a tar archive of the candidate tree into dir, which must exist and be empty. It accepts regular files and
// directories only; a symlink, a hard link, a device, an absolute or parent-relative name, a name that repeats and an archive
// over the limits are refused, so the archive cannot reach outside dir. Modes are normalised: the files are the provider's
// and are never executable.
func Extract(archive []byte, dir string) error {
	if len(archive) > maxCandidateBytes {
		return fmt.Errorf("%w: over %d bytes", ErrCandidate, maxCandidateBytes)
	}
	tr := tar.NewReader(bytes.NewReader(archive))
	seen := map[string]bool{}
	var files int
	var entries int
	var budget int64 = maxCandidateBytes
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCandidate, err)
		}
		// A sparse entry stores little and states a size to expand to: refuse every form of it, and count what each entry says it
		// holds against one budget for the whole archive.
		entries++
		if entries > maxFiles || h.Typeflag == tar.TypeGNUSparse || hasSparse(h.PAXRecords) || h.Size < 0 || h.Size > budget {
			return fmt.Errorf("%w: a sparse entry, too many entries or more bytes than the budget", ErrCandidate)
		}
		budget -= h.Size
		name := strings.TrimSuffix(h.Name, "/")
		if p, ok := cleanPath("", name); !ok || p != name || seen[name] {
			return fmt.Errorf("%w: a name is not a clean relative path or repeats", ErrCandidate)
		}
		seen[name] = true
		target := filepath.Join(dir, filepath.FromSlash(name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return fmt.Errorf("%w: %w", ErrCandidate, err)
			}
		case tar.TypeReg:
			files++
			if files > maxFiles || h.Size > maxFileSize {
				return fmt.Errorf("%w: too many or too large files", ErrCandidate)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return fmt.Errorf("%w: %w", ErrCandidate, err)
			}
			if err := writeFile(target, io.LimitReader(tr, maxFileSize+1)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: an entry is neither a file nor a directory", ErrCandidate)
		}
	}
}

func writeFile(path string, r io.Reader) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // path is under the private extraction directory, joined from a validated name
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCandidate, err)
	}
	n, err := io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil || n > maxFileSize {
		return fmt.Errorf("%w: a file cannot be written or is too large", ErrCandidate)
	}
	return nil
}

func hasSparse(pax map[string]string) bool {
	for k := range pax {
		if strings.HasPrefix(k, "GNU.sparse.") {
			return true
		}
	}
	return false
}

// ignoreFiles are the names a candidate can use to ask a scanning tool to skip files. The tools are run so that none is read; a
// candidate that carries one is reported, as a directive the tool was told to ignore.
//
//nolint:gochecknoglobals // read-only table
var ignoreFiles = map[string]string{".git": "ignore-file", ".gitignore": "gitignore", ".semgrepignore": "ignore-file", ".ignore": "ignore-file", ".ruffignore": "ignore-file"}

// allowedHooks are the pytest hooks a candidate may define: they shape collection and options and cannot rewrite an outcome. Every
// other hook (pytest_runtest_makereport, pytest_collection_modifyitems, pytest_runtest_call, ...) is reported, so the list names
// what is allowed, not what is denied.
//
//nolint:gochecknoglobals // read-only table
var allowedHooks = map[string]bool{"pytest_addoption": true, "pytest_generate_tests": true}

var (
	reMypyIgnore = regexp.MustCompile(`#\s*type:\s*ignore`)
	reMypyInline = regexp.MustCompile(`^\s*#\s*mypy:`)
	reSkip       = regexp.MustCompile(`pytest\.mark\.(skip|skipif|xfail)\b|pytest\.(skip|xfail)\(`)
	reHookimpl   = regexp.MustCompile(`\bhookimpl\b|^\s*pytest_plugins\s*=`)
	reHookDef    = regexp.MustCompile(`^\s*(?:async\s+)?def (pytest_\w+)`)
)

// lineDirectives returns the names of the directives of check on one line of a .py or .pyi file.
func lineDirectives(check, line string) []string {
	var out []string
	switch check {
	case checkMypy:
		if reMypyIgnore.MatchString(line) {
			out = append(out, "type-ignore")
		}
		if reMypyInline.MatchString(line) {
			out = append(out, "mypy-inline-config")
		}
	case checkPytest:
		if reSkip.MatchString(line) {
			out = append(out, "pytest-skip")
		}
		if reHookimpl.MatchString(line) {
			out = append(out, "pytest-hook")
		}
		if m := reHookDef.FindStringSubmatch(line); len(m) > 1 && !allowedHooks[m[1]] {
			out = append(out, "pytest-hook")
		}
	}
	return out
}

// ScanDirectives lists what the candidate carries that could hide a violation from check, in path and line order: the ignore
// files and the .git directory a scanning tool would honour, and the inline directives of mypy and pytest in every .py and .pyi
// file (conftest.py and stubs included).
func ScanDirectives(dir, check string) ([]Directive, error) {
	var out []Directive
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		if name, ok := ignoreFiles[d.Name()]; ok && rel != "." {
			out = append(out, Directive{Name: name, Path: filepath.ToSlash(rel), Line: 0})
		}
		if d.IsDir() || (!strings.HasSuffix(p, ".py") && !strings.HasSuffix(p, ".pyi")) {
			return nil
		}
		f, err := os.Open(p) //nolint:gosec // under the private extraction directory
		if err != nil {
			return err
		}
		defer f.Close() //nolint:errcheck // read-only
		sc := bufio.NewScanner(io.LimitReader(f, maxFileSize))
		sc.Buffer(make([]byte, 64<<10), maxFileSize+1)
		for line := 1; sc.Scan(); line++ {
			for _, name := range lineDirectives(check, sc.Text()) {
				out = append(out, Directive{Name: name, Path: filepath.ToSlash(rel), Line: line})
			}
		}
		return sc.Err()
	})
	return out, err
}
