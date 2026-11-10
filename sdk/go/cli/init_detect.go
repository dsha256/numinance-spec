package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// Bounds on what init reads from the target repository.
const (
	maxHintBytes    = 64 << 10
	maxVersionBytes = 256
)

// defaultPythonVersion and defaultPackageManager are the founder's defaults: uv, with pytest, ruff and mypy.
const (
	defaultPythonVersion  = "3.12"
	defaultPackageManager = "uv"
)

var (
	pythonVersionRe = regexp.MustCompile(`^3\.\d{1,2}$`)
	// dotPythonVersionRe reads the first line of .python-version: 3.11 or 3.11.4, nothing else (pyenv names such as
	// "pypy3.10" or a virtualenv name are not a plain CPython version and are not guessed at).
	dotPythonVersionRe = regexp.MustCompile(`^(3\.\d{1,2})(\.\d+)?$`)
)

// packageManagers is the closed set the generated toolchain can name.
func packageManagers() []string { return []string{"uv", "pip", "poetry"} }

// hint is what the target directory says about the toolchain.
type hint struct {
	// PackageManager is "" when the directory says nothing.
	PackageManager string
	// Source names the file (and table) that said it.
	Source string
	// Conflict is non-empty when the directory names two managers.
	Conflict []string
	// PythonVersion and VersionSource come from .python-version; empty when absent or not understood.
	PythonVersion, VersionSource string
	// ManagerEvidence and VersionEvidence are the evidence kinds of the two findings, for "numi discover".
	ManagerEvidence, VersionEvidence evidence
}

// detectHints looks for pyproject.toml, uv.lock, poetry.lock, requirements.txt and .python-version at the top of the
// target repository. It never follows a link and reads at most maxHintBytes of a regular file. A missing file is no hint.
func detectHints(r *repoReader) (hint, error) {
	root := r.root
	var h hint
	type found struct {
		manager, source string
		ev              evidence
	}
	var hits []found

	for _, f := range []struct{ file, manager string }{{"uv.lock", "uv"}, {"poetry.lock", "poetry"}} {
		ok, err := regularFile(root, f.file)
		if err != nil {
			return hint{}, err
		}
		if ok {
			hits = append(hits, found{f.manager, f.file, evLockfile})
		}
	}
	pyproject := r.text("pyproject.toml", maxHintBytes)
	hasPyproject := pyproject != nil
	if hasPyproject {
		if bytes.Contains(pyproject, []byte("[tool.poetry]")) || bytes.Contains(pyproject, []byte("[tool.poetry.")) {
			hits = append(hits, found{"poetry", "pyproject.toml [tool.poetry]", evPyprojectTable})
		}
		if bytes.Contains(pyproject, []byte("[tool.uv]")) || bytes.Contains(pyproject, []byte("[tool.uv.")) {
			hits = append(hits, found{"uv", "pyproject.toml [tool.uv]", evPyprojectTable})
		}
	}
	managers := map[string]bool{}
	var sources []string
	for _, f := range hits {
		managers[f.manager] = true
		sources = append(sources, f.source)
	}
	switch {
	case len(managers) > 1:
		h.Conflict = sources
	case len(hits) > 0:
		h.PackageManager, h.Source, h.ManagerEvidence = hits[0].manager, strings.Join(sources, ", "), hits[0].ev
	default:
		if ok, err := regularFile(root, "requirements.txt"); err != nil {
			return hint{}, err
		} else if ok {
			h.PackageManager, h.Source, h.ManagerEvidence = "pip", "requirements.txt", evDependency
		} else if hasPyproject {
			h.PackageManager, h.Source, h.ManagerEvidence = defaultPackageManager, "pyproject.toml with no package manager table, the default", evNothingFound
		}
	}
	h.PythonVersion, h.VersionSource, h.VersionEvidence = pythonVersionHint(r, pyproject)
	return h, r.err
}

// regularFile reports whether name exists in root as a regular file. A link or a special file is not one and is not
// followed; anything other than not-exist while looking is an error.
func regularFile(root *os.Root, name string) (bool, error) {
	fi, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, Errorf(KindInternal, "cannot inspect %s", name)
	}
	return fi.Mode().IsRegular(), nil
}

// choice is a value init chose and where it came from, for the report.
type choice struct {
	Value, Source string
}

func (c choice) String() string { return fmt.Sprintf("%s (%s)", c.Value, c.Source) }
