package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Limits of one call: what the provider reads is untrusted and bounded before use.
const (
	// MaxConfigBytes bounds the config the host resolves: the scope, and the context (the base tree's paths and its Justfile).
	MaxConfigBytes = 4 << 20
	// maxBoundValue bounds the numbers of a scope: a bound above it is as good as none and is refused.
	maxBoundValue = 1 << 24
	maxScopePaths = 256
	// maxContextPaths bounds the paths of the base tree a config may carry.
	maxContextPaths = 200_000
	maxJustfile     = 1 << 20
	// MaxContextLinks bounds the (document, target) pairs of Context.Links.
	MaxContextLinks = 30_000
)

// ErrConfig is the error of a config that is not the shape the provider accepts.
var ErrConfig = errors.New("the config is not acceptable")

var baseRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// Config is the configuration a config_ref names: the only configuration the check honours. No file of the candidate is read for
// it, so an author cannot loosen the scope from the candidate.
type Config struct {
	// Check is the check to run; the provider has one, "repo".
	Check string `json:"check"`
	// Scope bounds what a candidate may change.
	Scope Scope `json:"scope"`
	// Context is the base tree the candidate is a change to, read from the protected base revision by the binding: the
	// links and the Justfile the changed documents are checked against.
	Context Context `json:"context"`
}

// Scope bounds what a candidate may change. Every bound is explicit: a scope that leaves one out is refused, because a missing
// bound would read as no bound.
type Scope struct {
	// AllowPaths are the paths a change may touch: an exact file, or a directory written with a trailing "/". At least one.
	AllowPaths []string `json:"allow_paths"`
	// ProtectedPaths are paths no change may touch even under an allowed path, matched without regard to case.
	ProtectedPaths []string `json:"protected_paths,omitempty"`
	// MaxFiles bounds the changed files, deletions included.
	MaxFiles *int `json:"max_files"`
	// MaxAddedLines bounds the lines of the added and modified files (LF, CR, CRLF, U+2028 and U+2029 each end a line). For a modified file it is the length of the new file, an
	// upper bound of the lines added, because the candidate carries the new content and not the old.
	MaxAddedLines *int `json:"max_added_lines"`
	// MaxAddedBytes bounds the bytes of the added and modified files, so a file of one enormous line cannot pass a bound on lines.
	MaxAddedBytes *int `json:"max_added_bytes"`
	// MaxDeletedFiles bounds the deleted files; zero forbids deletion.
	MaxDeletedFiles *int `json:"max_deleted_files"`
	// AllowModify permits a change to a file that exists in the base. False allows new files only; the lines a modification
	// removes are not measurable from the candidate, so a scope that must bound them forbids modification.
	AllowModify bool `json:"allow_modify"`
}

// Context is the base tree of the candidate.
type Context struct {
	// Base is the full commit id the paths and the Justfile are read at; it must be the base the candidate names.
	Base string `json:"base"`
	// Paths are the files of the base tree.
	Paths []string `json:"paths"`
	// Justfile is the text of the Justfile of the base tree, empty when it has none.
	Justfile string `json:"justfile,omitempty"`
	// Links maps a file of the base tree to the base Markdown documents that link to it, so a candidate that deletes the file is
	// caught by the documents it does not touch. The binding loader computes it with LinkTargets.
	Links map[string][]string `json:"links,omitempty"`
}

// ParseConfig decodes and validates config bytes: strict fields, bounded sizes and well-formed paths. A failure is ErrConfig.
func ParseConfig(b []byte) (Config, error) {
	if len(b) > MaxConfigBytes {
		return Config{}, fmt.Errorf("%w: over %d bytes", ErrConfig, MaxConfigBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var c Config
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("%w: trailing data after the document", ErrConfig)
	}
	if err := c.validate(); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.Check == "" || len(c.Check) > 64 {
		return errors.New("no check named")
	}
	s := &c.Scope
	if len(s.AllowPaths) == 0 {
		return errors.New("scope.allow_paths is empty: a scope that allows nothing is a mistake")
	}
	for name, v := range map[string][]string{"scope.allow_paths": s.AllowPaths, "scope.protected_paths": s.ProtectedPaths} {
		if len(v) > maxScopePaths {
			return fmt.Errorf("%s: more than %d entries", name, maxScopePaths)
		}
		for _, p := range v {
			if !validPath(strings.TrimSuffix(p, "/")) {
				return fmt.Errorf("%s: an entry is not a clean relative path", name)
			}
		}
	}
	for name, v := range map[string]*int{"max_files": s.MaxFiles, "max_added_lines": s.MaxAddedLines, "max_added_bytes": s.MaxAddedBytes, "max_deleted_files": s.MaxDeletedFiles} {
		if v == nil || *v < 0 || *v > maxBoundValue {
			return fmt.Errorf("scope.%s is missing or out of range", name)
		}
	}
	x := &c.Context
	if !baseRe.MatchString(x.Base) || len(x.Paths) > maxContextPaths || len(x.Justfile) > maxJustfile || !utf8.ValidString(x.Justfile) {
		return errors.New("context: the base is not a full commit id, or a bound is exceeded")
	}
	for _, p := range x.Paths {
		if !validPath(p) {
			return errors.New("context: a path is not a clean relative path")
		}
	}
	pairs := 0
	for to, from := range x.Links {
		pairs += len(from)
		if !validPath(to) || pairs > MaxContextLinks {
			return errors.New("context: a link target is not a clean relative path, or there are too many links")
		}
		for _, f := range from {
			if !validPath(f) {
				return errors.New("context: a linking document is not a clean relative path")
			}
		}
	}
	return nil
}
