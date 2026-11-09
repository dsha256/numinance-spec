// Package extpkg reads an extension package archive and computes the three values the lockfile pins (design section 17,
// section 8 row Lockfile): the content digest a manifest declares, the h1 hash of the unpacked tree and the zh hash of the
// archive. It is public so that an extension author can compute the digest to declare, and so that "numi extension verify"
// and the install flow of the Engine compute the same values (a test in the Engine pins that they do).
//
// A package is a zip archive of regular files; manifest.yaml sits at its root. The archive is untrusted input: Unpack
// refuses paths that escape or confuse an install, links, duplicates, and anything over the limits.
package extpkg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
)

// ManifestFile is the name of the manifest at the root of a package. The content digest excludes it because it carries
// the digest; the signature over the whole artifact covers it, and h1 covers it.
const ManifestFile = "manifest.yaml"

// File is one regular file of an unpacked package.
type File struct {
	// Path is slash separated, relative and clean.
	Path string
	Data []byte
}

// Limits bound an archive being unpacked.
type Limits struct {
	MaxFiles     int
	MaxFileBytes int64
	MaxTotal     int64
}

// DefaultLimits returns the bounds the install flow uses.
func DefaultLimits() Limits {
	return Limits{MaxFiles: 4096, MaxFileBytes: 64 << 20, MaxTotal: 256 << 20}
}

// ErrArchive is wrapped by every refusal of an archive.
var ErrArchive = errors.New("extpkg: archive rejected")

func bad(rule string) error { return fmt.Errorf("%w: %s", ErrArchive, rule) }

// Unpack reads the regular files of a zip archive in memory, sorted by path. Directory entries are skipped. Sizes are
// measured while reading, never taken from the headers.
func Unpack(archive []byte, lim Limits) ([]File, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, bad("not a zip archive")
	}
	if len(zr.File) > lim.MaxFiles {
		return nil, bad("too many entries")
	}
	var (
		files []File
		total int64
		seen  = map[string]bool{}
	)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			return nil, bad("an entry is not a regular file")
		}
		if err := validPath(f.Name); err != nil {
			return nil, err
		}
		if seen[strings.ToLower(f.Name)] {
			return nil, bad("duplicate or colliding path")
		}
		seen[strings.ToLower(f.Name)] = true
		data, err := readBounded(f, min(lim.MaxFileBytes, lim.MaxTotal-total))
		if err != nil {
			return nil, err
		}
		total += int64(len(data))
		files = append(files, File{Path: f.Name, Data: data})
	}
	if err := checkTree(seen); err != nil {
		return nil, err
	}
	slices.SortFunc(files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return files, nil
}

// checkTree refuses a file that is also a directory of another file ("a" and "a/b"): on disk one of the two would replace
// or shadow the other. folded holds the lower-cased paths, so the check is case-insensitive as well.
func checkTree(folded map[string]bool) error {
	for p := range folded {
		for i := 0; i < len(p); i++ {
			if p[i] == '/' && folded[p[:i]] {
				return bad("a file is also a directory of another file")
			}
		}
	}
	return nil
}

func readBounded(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, bad("an entry cannot be read")
	}
	defer func() { _ = rc.Close() }() // read side of an in-memory archive: nothing to lose on close
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, bad("an entry cannot be read")
	}
	if int64(len(data)) > limit {
		return nil, bad("an entry or the total is over the size limit")
	}
	return data, nil
}

func validPath(p string) error {
	if p == "" || len(p) > 1024 || strings.HasPrefix(p, "/") || strings.ContainsRune(p, '\\') {
		return bad("a path is empty, absolute, too long or holds a backslash")
	}
	if strings.ContainsFunc(p, func(r rune) bool { return unicode.IsControl(r) || r == 0x7f }) {
		return bad("a path holds a control character")
	}
	// ASCII only: two spellings of one name (NFC and NFD, or a case fold beyond ASCII) cannot then collide on a
	// normalizing or case-folding file system, and the check above needs no Unicode tables.
	if strings.ContainsFunc(p, func(r rune) bool { return r > unicode.MaxASCII }) {
		return bad("a path holds a non-ASCII character")
	}
	for seg := range strings.SplitSeq(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return bad("a path has an empty, . or .. segment")
		}
	}
	return nil
}

// ArchiveHash is the zh pin: "zh:" plus the lowercase hex sha256 of the archive as downloaded.
func ArchiveHash(archive []byte) string {
	sum := sha256.Sum256(archive)
	return "zh:" + hex.EncodeToString(sum[:])
}

// TreeHash is the h1 pin: "h1:" plus the standard base64 of the sha256 of the sorted lines
// "<hex sha256 of file>  <path>\n", the Go module tree hash. It covers every file, manifest.yaml included.
func TreeHash(files []File) string {
	sum := sha256.Sum256(summary(files, nil))
	return "h1:" + base64.StdEncoding.EncodeToString(sum[:])
}

// ContentDigest is the digest a manifest declares and the lockfile pins as digest: "sha256:" plus the hex sha256 of the
// same summary with manifest.yaml excluded.
func ContentDigest(files []File) string {
	sum := sha256.Sum256(summary(files, func(p string) bool { return p == ManifestFile }))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func summary(files []File, skip func(string) bool) []byte {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	var b bytes.Buffer
	for _, f := range sorted {
		if skip != nil && skip(f.Path) {
			continue
		}
		fmt.Fprintf(&b, "%x  %s\n", sha256.Sum256(f.Data), f.Path)
	}
	return b.Bytes()
}
