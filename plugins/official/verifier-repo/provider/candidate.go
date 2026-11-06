package provider

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// The candidate is a tar archive of the change, as the host-run executor path cuts it: the first entry is manifest.json, which
// names the base the candidate is a change to and every changed path with its status, and each added, modified or retyped
// file follows under "files/". A deletion is in the manifest only. The archive is untrusted: it is read in memory, never
// extracted, and every name, size and count is bounded before it is used.
const (
	// ManifestName is the first entry of a candidate archive.
	ManifestName = "manifest.json"
	// FilesPrefix starts the name of every file entry, so no path of the change collides with the manifest.
	FilesPrefix = "files/"
	// ManifestVersion is the one manifest version the provider reads.
	ManifestVersion = 1
)

// Limits of a candidate.
const (
	// MaxCandidateBytes is the most the host serves for one reference.
	MaxCandidateBytes = 16 << 20
	maxEntries        = 20_000
	maxManifestBytes  = 4 << 20
	maxFileBytes      = 8 << 20
	// maxDocBytes bounds a document the checks read in full (a Markdown file, the Justfile).
	maxDocBytes = 1 << 20
)

var (
	// ErrCandidate is the error of a candidate archive that is not of the shape the provider accepts.
	ErrCandidate = errors.New("the candidate artifact is not an acceptable archive")
	// ErrTooLarge is the error of a candidate over a bound: bytes, entries or the size of one file.
	ErrTooLarge = errors.New("the candidate artifact is over a limit")
)

// Statuses of a changed path, as git writes them.
const (
	statusAdded    = "A"
	statusModified = "M"
	statusDeleted  = "D"
	statusTypeChg  = "T"
)

const (
	modeRegular    = "100644"
	modeExecutable = "100755"
	modeSymlink    = "120000"
)

var modeRe = regexp.MustCompile(`^[0-7]{6}$`)

// manifestEntry is one changed path of the manifest.
type manifestEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Mode   string `json:"mode,omitempty"`
}

// manifest is the document the candidate opens with.
type manifest struct {
	Version int             `json:"version"`
	Base    string          `json:"base"`
	Entries []manifestEntry `json:"entries"`
}

// Change is one changed path of a candidate.
type Change struct {
	Path   string
	Status string
	// Symlink is true for a symbolic link; it has no content and its target is never followed.
	Symlink bool
	// Lines is the number of lines of the new content; zero for a deletion and a link.
	Lines int
	// Bytes is the size of the new content; zero for a deletion and a link.
	Bytes int64
	// Content is the new content of a Markdown file or the Justfile, which the checks read; nil for any other path.
	Content []byte
}

// Candidate is a parsed candidate: the base it is a change to and its changed paths in path order.
type Candidate struct {
	Base    string
	Changes []Change
}

// wantsContent reports whether the checks read the content of the path.
func wantsContent(p string) bool { return p == justfileName || IsMarkdown(p) }

// IsMarkdown reports whether the path names a Markdown document: .md or .markdown in any case, as a file system that folds case
// would see it.
func IsMarkdown(p string) bool {
	l := strings.ToLower(p)
	return strings.HasSuffix(l, ".md") || strings.HasSuffix(l, ".markdown")
}

// ParseCandidate reads a candidate archive. It refuses, with ErrCandidate, an archive that does not open with a valid manifest,
// a name that is not a clean relative path (an absolute name, a "..", a backslash), a repeated name, an entry that is not a
// regular file or a symbolic link, a sparse entry, a file the manifest does not list or lists and the archive lacks, and a
// mode that does not match the entry. It refuses, with ErrTooLarge, an archive over MaxCandidateBytes, more than 20000 entries
// and a file over its bound. Nothing is written to disk.
func ParseCandidate(archive []byte) (Candidate, error) {
	if len(archive) > MaxCandidateBytes {
		return Candidate{}, fmt.Errorf("%w: over %d bytes", ErrTooLarge, MaxCandidateBytes)
	}
	tr := tar.NewReader(bytes.NewReader(archive))
	m, err := readManifest(tr)
	if err != nil {
		return Candidate{}, err
	}
	byPath := make(map[string]*Change, len(m.Entries))
	var out Candidate
	out.Base = m.Base
	out.Changes = make([]Change, 0, len(m.Entries))
	for _, e := range m.Entries {
		out.Changes = append(out.Changes, Change{Path: e.Path, Status: e.Status, Symlink: e.Mode == modeSymlink})
	}
	for i := range out.Changes {
		byPath[out.Changes[i].Path] = &out.Changes[i]
	}
	modes := make(map[string]string, len(m.Entries))
	for _, e := range m.Entries {
		modes[e.Path] = e.Mode
	}
	seen := map[string]bool{}
	budget := int64(MaxCandidateBytes)
	for entries := 1; ; entries++ {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Candidate{}, fmt.Errorf("%w: %w", ErrCandidate, err)
		}
		if entries > maxEntries {
			return Candidate{}, fmt.Errorf("%w: more than %d entries", ErrTooLarge, maxEntries)
		}
		rel, ok := strings.CutPrefix(h.Name, FilesPrefix)
		if !ok || !validPath(rel) || seen[rel] || hasSparse(h) {
			return Candidate{}, fmt.Errorf("%w: an entry name is outside files/, not a clean relative path, repeated, or sparse", ErrCandidate)
		}
		seen[rel] = true
		c, listed := byPath[rel]
		if !listed || c.Status == statusDeleted {
			return Candidate{}, fmt.Errorf("%w: a file the manifest does not list as present", ErrCandidate)
		}
		if err := readEntry(tr, h, c, modes[rel], &budget); err != nil {
			return Candidate{}, err
		}
	}
	for _, c := range out.Changes {
		if c.Status != statusDeleted && !seen[c.Path] {
			return Candidate{}, fmt.Errorf("%w: the manifest lists a file the archive lacks", ErrCandidate)
		}
	}
	return out, nil
}

func hasSparse(h *tar.Header) bool {
	if h.Typeflag == tar.TypeGNUSparse {
		return true
	}
	for k := range h.PAXRecords {
		if strings.HasPrefix(k, "GNU.sparse.") {
			return true
		}
	}
	return false
}

// readManifest reads and validates the first entry.
func readManifest(tr *tar.Reader) (manifest, error) {
	h, err := tr.Next()
	if err != nil {
		return manifest{}, fmt.Errorf("%w: no manifest: %w", ErrCandidate, err)
	}
	if h.Name != ManifestName || h.Typeflag != tar.TypeReg || hasSparse(h) {
		return manifest{}, fmt.Errorf("%w: the first entry is not %s", ErrCandidate, ManifestName)
	}
	if h.Size < 0 || h.Size > maxManifestBytes {
		return manifest{}, fmt.Errorf("%w: the manifest is over %d bytes", ErrTooLarge, maxManifestBytes)
	}
	raw, err := io.ReadAll(io.LimitReader(tr, h.Size))
	if err != nil {
		return manifest{}, fmt.Errorf("%w: %w", ErrCandidate, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m manifest
	if err := dec.Decode(&m); err != nil {
		return manifest{}, fmt.Errorf("%w: the manifest: %w", ErrCandidate, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return manifest{}, fmt.Errorf("%w: trailing data after the manifest", ErrCandidate)
	}
	if err := m.validate(); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func (m manifest) validate() error {
	if m.Version != ManifestVersion || !baseRe.MatchString(m.Base) {
		return fmt.Errorf("%w: the manifest has an unknown version or no full base commit id", ErrCandidate)
	}
	if len(m.Entries) > maxEntries {
		return fmt.Errorf("%w: more than %d changed paths", ErrTooLarge, maxEntries)
	}
	seen := make(map[string]bool, len(m.Entries))
	for _, e := range m.Entries {
		if !validPath(e.Path) || seen[e.Path] {
			return fmt.Errorf("%w: a manifest path is not a clean relative path or repeats", ErrCandidate)
		}
		seen[e.Path] = true
		switch e.Status {
		case statusDeleted:
			if e.Mode != "" {
				return fmt.Errorf("%w: a deletion has a mode", ErrCandidate)
			}
		case statusAdded, statusModified, statusTypeChg:
			if !modeRe.MatchString(e.Mode) || (e.Mode != modeRegular && e.Mode != modeExecutable && e.Mode != modeSymlink) {
				return fmt.Errorf("%w: a changed path has a mode other than a file or a link", ErrCandidate)
			}
		default:
			return fmt.Errorf("%w: a changed path has an unknown status", ErrCandidate)
		}
	}
	return nil
}

// readEntry reads one file entry into c: its line count, its content when the checks read it, and its kind against the
// manifest's mode. budget is what the archive may still hold.
func readEntry(tr *tar.Reader, h *tar.Header, c *Change, mode string, budget *int64) error {
	if h.Size < 0 || h.Size > maxFileBytes || h.Size > *budget {
		return fmt.Errorf("%w: a file is over its bound or the archive's budget", ErrTooLarge)
	}
	*budget -= h.Size
	switch h.Typeflag {
	case tar.TypeSymlink:
		if mode != modeSymlink || h.Size != 0 || len(h.Linkname) > maxPath {
			return fmt.Errorf("%w: a link entry does not match its manifest mode", ErrCandidate)
		}
		return nil
	case tar.TypeReg:
		if mode == modeSymlink {
			return fmt.Errorf("%w: a file entry where the manifest says link", ErrCandidate)
		}
	default:
		return fmt.Errorf("%w: an entry is neither a file nor a link", ErrCandidate)
	}
	c.Bytes = h.Size
	if wantsContent(c.Path) {
		if h.Size > maxDocBytes {
			return fmt.Errorf("%w: a document is over %d bytes", ErrTooLarge, maxDocBytes)
		}
		body, err := io.ReadAll(io.LimitReader(tr, h.Size))
		if err != nil || int64(len(body)) != h.Size {
			return fmt.Errorf("%w: a file is cut short", ErrCandidate)
		}
		c.Content, c.Lines = body, countLines(body)
		return nil
	}
	n, err := countStream(tr, h.Size)
	if err != nil {
		return err
	}
	c.Lines = n
	return nil
}

// lineCounter counts lines the way a reader of the file would see them: LF, CR, CRLF, U+2028 and U+2029 each end a line, and a
// last line with no ending counts too. It is fed in chunks, so a break split across two reads is still one.
type lineCounter struct {
	n            int
	any, endedOn bool
	prev1, prev2 byte
}

func (c *lineCounter) add(b []byte) {
	for _, x := range b {
		c.any = true
		switch {
		case x == '\n':
			if c.prev1 != '\r' { // the CR of a CRLF was counted
				c.n++
			}
			c.endedOn = true
		case x == '\r':
			c.n++
			c.endedOn = true
		case (x == 0xA8 || x == 0xA9) && c.prev1 == 0x80 && c.prev2 == 0xE2:
			c.n++
			c.endedOn = true
		default:
			c.endedOn = false
		}
		c.prev2, c.prev1 = c.prev1, x
	}
}

func (c *lineCounter) lines() int {
	if c.any && !c.endedOn {
		return c.n + 1
	}
	return c.n
}

// countLines is the number of lines of b.
func countLines(b []byte) int {
	var c lineCounter
	c.add(b)
	return c.lines()
}

// countStream counts the lines of the next size bytes of r without keeping them.
func countStream(r io.Reader, size int64) (int, error) {
	buf := make([]byte, 32<<10)
	var c lineCounter
	var read int64
	for read < size {
		chunk := buf[:min(int64(len(buf)), size-read)]
		k, err := r.Read(chunk)
		c.add(chunk[:k])
		read += int64(k)
		if errors.Is(err, io.EOF) && read < size {
			return 0, fmt.Errorf("%w: a file is cut short", ErrCandidate)
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, fmt.Errorf("%w: %w", ErrCandidate, err)
		}
		if k == 0 && err == nil {
			return 0, fmt.Errorf("%w: a file cannot be read", ErrCandidate)
		}
	}
	return c.lines(), nil
}
