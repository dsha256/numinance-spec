package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/numinance/numinance-spec/sdk/go/cli"
	"github.com/numinance/numinance-spec/sdk/go/kindyaml"
)

const (
	configDir  = ".numinance"
	maxDocs    = 256
	maxEntries = 4096
	// maxTotalBytes bounds all documents of one run, so 256 documents of the
	// largest size cannot be held at once.
	maxTotalBytes = 16 << 20
)

// workingTree is the Source of the open build: it reads the kind documents
// under .numinance/ of directories on disk. It reads the working tree, not a
// commit, so it cannot tell committed from uncommitted text; the engine build
// reads a commit from a hardened mirror, and --rev is refused here. Directories
// are opened with os.Root, so no path leaves them. A symbolic link, anywhere on
// the way to a document or as a document, and any file that is not regular are
// refused. The entry, document and byte counts are bounded across all trees.
type workingTree struct{}

// budget is what one Snapshot may still read.
type budget struct {
	entries, docs, bytes int
}

func (workingTree) Snapshot(ctx context.Context, req cli.Request) (cli.Snapshot, error) {
	if req.Rev != "" {
		return cli.Snapshot{}, cli.Errorf(cli.KindUsage, "the open build reads the working tree; --rev needs the engine build")
	}
	snap := cli.Snapshot{Revision: "working tree (uncommitted changes included)"}
	left := budget{entries: maxEntries, docs: maxDocs, bytes: maxTotalBytes}
	read := func(scope, dir, sub string) error {
		docs, err := readTree(ctx, dir, sub, scope, &left)
		snap.Documents = append(snap.Documents, docs...)
		return err
	}
	if req.OrgDir != "" {
		if err := read("org", req.OrgDir, configDir); err != nil {
			return cli.Snapshot{}, err
		}
	}
	if err := read("repo", req.RepoDir, configDir); err != nil {
		return cli.Snapshot{}, err
	}
	if req.Component != "" {
		if !plainComponent(req.Component) {
			return cli.Snapshot{}, cli.Errorf(cli.KindInvalid, "--component must be a plain repository-relative path")
		}
		if err := read("component", req.RepoDir, req.Component+"/"+configDir); err != nil {
			return cli.Snapshot{}, err
		}
	}
	return snap, nil
}

// plainComponent accepts slash-joined elements, none empty, "." or "..", none
// named like the config directory.
func plainComponent(c string) bool {
	if !fs.ValidPath(c) || c == "." || strings.ContainsAny(c, `\:`) {
		return false
	}
	for e := range strings.SplitSeq(c, "/") {
		if strings.EqualFold(e, configDir) {
			return false
		}
	}
	return true
}

// readTree reads the YAML files below sub in dir. A missing sub is an empty
// tree (a repository without kinds); a missing dir is NotFound.
func readTree(ctx context.Context, dir, sub, scope string, left *budget) ([]cli.Document, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, cli.Errorf(cli.KindNotFound, "%s directory not found", scope)
		}
		return nil, cli.Errorf(cli.KindInvalid, "%s directory cannot be opened", scope)
	}
	defer func() { _ = root.Close() }()
	return readRoot(ctx, root, sub, scope, left, false)
}

// readRoot reads the YAML files below sub in an open root. With exactExt, a
// file name that yamlLike reports (a name mentioning .yaml or .yml without
// ending exactly in lowercase .yaml or .yml) is an error instead of a silently
// ignored file.
func readRoot(ctx context.Context, root *os.Root, sub, scope string, left *budget, exactExt bool) ([]cli.Document, error) {
	present, err := plainDirs(root, sub)
	if err != nil {
		return nil, cli.Errorf(cli.KindInvalid, "%s tree: a link or non-directory on the path to %s", scope, configDir)
	}
	if !present {
		return nil, nil
	}
	var docs []cli.Document
	err = fs.WalkDir(root.FS(), sub, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		left.entries--
		if left.entries < 0 {
			return cli.Errorf(cli.KindInvalid, "more than %d entries", maxEntries)
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return cli.Errorf(cli.KindInvalid, "%s tree has a file that is not regular (a link or special file)", scope)
		}
		if !strings.HasSuffix(p, ".yaml") && !strings.HasSuffix(p, ".yml") {
			if exactExt && yamlLike(p) {
				return cli.Errorf(cli.KindInvalid, "%s: a file name mentioning .yaml or .yml must end in lowercase .yaml or .yml", p)
			}
			return nil
		}
		left.docs--
		if left.docs < 0 {
			return cli.Errorf(cli.KindInvalid, "more than %d kind documents", maxDocs)
		}
		b, err := readRegular(root, p, nil)
		if err != nil {
			return err
		}
		left.bytes -= len(b)
		if left.bytes < 0 {
			return cli.Errorf(cli.KindInvalid, "kind documents are larger than %d bytes together", maxTotalBytes)
		}
		docs = append(docs, cli.Document{Scope: scope, Path: path.Clean(p), Data: b})
		return nil
	})
	if err != nil {
		var ce *cli.Error
		if errors.As(err, &ce) || ctx.Err() != nil {
			return nil, err
		}
		return nil, cli.Errorf(cli.KindInvalid, "%s tree cannot be read", scope)
	}
	return docs, nil
}

// plainDirs reports whether every element of sub exists as a real directory. A
// missing element is (false, nil); a link or a non-directory is an error.
func plainDirs(root *os.Root, sub string) (bool, error) {
	elems := strings.Split(sub, "/")
	for i := range elems {
		fi, err := root.Lstat(strings.Join(elems[:i+1], "/"))
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !fi.IsDir() {
			return false, errors.New("not a directory")
		}
	}
	return true, nil
}

// readRegular reads at most kindyaml.MaxBytes+1 bytes of a file, after checking
// on the open descriptor that it is regular and, when want is set, that it is
// the file want describes.
func readRegular(root *os.Root, p string, want fs.FileInfo) ([]byte, error) {
	f, err := root.OpenFile(p, os.O_RDONLY|openFlags, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, cli.Errorf(cli.KindInvalid, "a kind document is not a regular file")
	}
	if want != nil && !os.SameFile(want, fi) {
		return nil, cli.Errorf(cli.KindInvalid, "a kind document changed while it was opened")
	}
	return io.ReadAll(io.LimitReader(f, kindyaml.MaxBytes+1))
}

// yamlLike reports a file name that mentions .yaml or .yml (any letter case)
// but does not end exactly in lowercase .yaml or .yml, so the loader would not
// read it: .YAML, .Yml, .yaml.bak, .yaml-old, .yamlx, a trailing space. Names
// that do not mention YAML at all (README.md~) are not YAML-like.
func yamlLike(p string) bool {
	low := strings.ToLower(path.Base(p))
	return strings.Contains(low, ".yaml") || strings.Contains(low, ".yml")
}
