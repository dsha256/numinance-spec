package evidenceverify

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxEntries           = 64
	pathRecords          = "records.ndjson"
	pathSummary          = "summary.json"
	pathIndex            = "export-index.json"
	pathManifest         = "bundle-manifest.json"
	pathAttest           = "attestation.json"
	pathAttestCP         = "attestation-checkpoint.json"
	pathGap              = "attestation-gap.json"
	pathHeadCP           = "head-checkpoint.json"
	pathHeadProof        = "head-inclusion-proof.json"
	pathDiff             = "diff.digest"
	pathToolEvents       = "tool-events.ndjson"
	pathChange           = "change.json"
	pathSigners          = "provenance/signers.json"
	pathProvenance       = "provenance/provenance.json"
	pathExecutorManifest = "provenance/executor-manifest.json"
	pathVerdicts         = "verification/verdicts.json"
	pathReview           = "review/review.json"
)

// The seventeen file names of evidence/v1 (bundle-layout.md section 1).
func knownPaths() []string {
	return []string{
		pathRecords, pathSummary, pathIndex, pathSigners, pathProvenance, pathVerdicts, pathReview, pathDiff,
		pathToolEvents, pathChange, pathAttest, pathAttestCP, pathGap, pathManifest, pathHeadCP, pathHeadProof, pathExecutorManifest,
	}
}

// The parts of the export part list, in order (bundle-layout.md section 3).
func designParts() []string {
	return []string{
		"change.json", "attestation.json", "records.ndjson", "tool-events.ndjson", "diff.digest",
		"verification/", "review/", "provenance/", "deployment.json", "observation.json", "rollback.json", "summary.json",
	}
}

func isKnownPath(p string) bool {
	return slices.Contains(knownPaths(), p)
}

func fileLimit(name string) int {
	if name == pathRecords || name == pathToolEvents {
		return maxBigFile
	}
	return maxDoc
}

var errTooLarge = errors.New("file over its size bound")

type fileRead struct {
	data []byte
	err  error
}

// bundle is an export directory opened read-only through an os.Root.
type bundle struct {
	root  *os.Root
	files map[string]bool // slash-separated relative path of every regular file
	cache map[string]*fileRead
}

func (b *bundle) close() { _ = b.root.Close() }

func (b *bundle) has(name string) bool { return b.files[name] }

// holdsDir reports whether any file is present inside a part directory.
func (b *bundle) holdsDir(dir string) bool {
	for f := range b.files {
		if strings.HasPrefix(f, dir) {
			return true
		}
	}
	return false
}

// read returns the file's bytes, at most its limit. A file over the limit is
// refused before the first read, and one that grows while it is read fails at
// the limit.
func (b *bundle) read(name string) ([]byte, error) {
	if c, ok := b.cache[name]; ok {
		return c.data, c.err
	}
	data, err := b.readFile(name, fileLimit(name))
	b.cache[name] = &fileRead{data, err}
	return data, err
}

func (b *bundle) readFile(name string, limit int) ([]byte, error) {
	f, err := b.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if st.Size() > int64(limit) {
		return nil, errTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

func validName(name string) bool {
	if len(name) < 1 || len(name) > 255 || name == "." || name == ".." || !utf8.ValidString(name) {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '/' || c == '\\' || c == ':' || c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

// openBundle opens the directory at path and lists it (bundle-layout.md
// section 1). A non-empty refusal means the listing found something that is
// refused as directory_refused; the error is for a path that is itself refused
// (exit 3), missing (4) or unreadable (7).
func openBundle(path string) (*bundle, string, error) {
	// Clean first: Lstat of "link/", "link/." or "link//" follows the link, so the
	// spelling must not decide whether a link is refused.
	path = filepath.Clean(path)
	fi, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, "", errf(KindNotFound, "the directory does not exist")
	case err != nil:
		return nil, "", &Error{Kind: KindUnavailable, Msg: "the directory cannot be read", Err: err}
	case fi.Mode()&os.ModeSymlink != 0:
		return nil, "", errf(KindInvalid, "the path is a symbolic link")
	case !fi.IsDir():
		return nil, "", errf(KindInvalid, "the path is not a directory")
	}
	if st, serr := os.Stat(path); serr != nil || !os.SameFile(fi, st) {
		return nil, "", errf(KindInvalid, "the path does not name the directory it resolves to")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, "", &Error{Kind: KindUnavailable, Msg: "the directory cannot be opened", Err: err}
	}
	b := &bundle{root: root, files: map[string]bool{}, cache: map[string]*fileRead{}}
	top, err := root.Open(".")
	if err != nil {
		b.close()
		return nil, "", &Error{Kind: KindUnavailable, Msg: "the directory cannot be opened", Err: err}
	}
	defer func() { _ = top.Close() }()
	if st, err := top.Stat(); err != nil || !os.SameFile(fi, st) {
		b.close()
		return nil, "", errf(KindInvalid, "the directory changed while it was opened")
	}
	refusal, err := b.list(top)
	if err != nil {
		b.close()
		return nil, "", err
	}
	return b, refusal, nil
}

func readNames(d *os.File) ([]os.DirEntry, error) {
	ents, err := d.ReadDir(maxEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, &Error{Kind: KindUnavailable, Msg: "the directory cannot be listed", Err: err}
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
	return ents, nil
}

func (b *bundle) list(top *os.File) (string, error) {
	ents, err := readNames(top)
	if err != nil {
		return "", err
	}
	if len(ents) > maxEntries {
		return "the directory holds more than 64 entries", nil
	}
	total := 0
	for _, e := range ents {
		if !validName(e.Name()) {
			return "an entry has a name that is not a plain file name", nil
		}
		switch {
		case e.Type().IsRegular():
			b.files[e.Name()] = true
			total++
		case e.IsDir() && isPartDir(e.Name()):
			sub, err := b.root.Open(e.Name())
			if err != nil {
				return "", &Error{Kind: KindUnavailable, Msg: "a directory cannot be opened", Err: err}
			}
			inner, err := readNames(sub)
			_ = sub.Close()
			if err != nil {
				return "", err
			}
			if len(inner) > maxEntries {
				return "a part directory holds more than 64 entries", nil
			}
			for _, ie := range inner {
				if !validName(ie.Name()) {
					return "an entry has a name that is not a plain file name", nil
				}
				if !ie.Type().IsRegular() {
					return "a part directory holds an entry that is not a regular file", nil
				}
				b.files[e.Name()+"/"+ie.Name()] = true
				total++
			}
		default:
			return "the directory holds a link, a directory or another entry that is not a regular file", nil
		}
		if total > maxEntries {
			return "the export holds more than 64 files", nil
		}
	}
	return "", nil
}

func isPartDir(n string) bool {
	return n == "verification" || n == "review" || n == "provenance"
}
