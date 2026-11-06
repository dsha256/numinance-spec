package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/numinance/numinance-spec/sdk/go/cli"
)

// fileTree is the Source behind "numi validate --files": a YAML file or a
// directory of them, such as numinance-spec/schemas/examples. It reads with the
// same limits and refusals as workingTree: os.Root, no links, regular files
// only, and the entry, document and byte bounds.
type fileTree struct {
	// lstat stands in for os.Lstat in tests, to simulate a swap after the check; nil means os.Lstat.
	lstat func(string) (fs.FileInfo, error)
}

func (t fileTree) Snapshot(ctx context.Context, req cli.Request) (cli.Snapshot, error) {
	snap := cli.Snapshot{Revision: "files"}
	left := budget{entries: maxEntries, docs: maxDocs, bytes: maxTotalBytes}
	p := filepath.Clean(req.RepoDir)
	lstat := os.Lstat
	if t.lstat != nil {
		lstat = t.lstat
	}
	fi, err := lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return snap, cli.Errorf(cli.KindNotFound, "path not found")
	case err != nil:
		return snap, cli.Errorf(cli.KindInvalid, "path cannot be read")
	case fi.IsDir():
		root, err := openSame(p, fi)
		if err != nil {
			return snap, err
		}
		defer func() { _ = root.Close() }()
		docs, err := readRoot(ctx, root, ".", "file", &left, true)
		snap.Documents = docs
		return snap, err
	case !fi.Mode().IsRegular():
		return snap, cli.Errorf(cli.KindInvalid, "path is not a regular file or a directory (a link is refused)")
	}
	if ext := filepath.Ext(fi.Name()); ext != ".yaml" && ext != ".yml" {
		return snap, cli.Errorf(cli.KindUsage, "a file must be named *.yaml or *.yml")
	}
	b, err := readSame(p, fi)
	if err != nil {
		return snap, err
	}
	name := filepath.Base(p)
	snap.Documents = []cli.Document{{Scope: "file", Path: name, Data: b}}
	return snap, nil
}

// openSame opens the directory p and checks that it is the one Lstat saw, so
// a link (a trailing slash included) or a swap between the Lstat and this open
// is refused. The caller walks the returned root, so nothing is reopened.
func openSame(p string, lstat fs.FileInfo) (*os.Root, error) {
	root, err := os.OpenRoot(p)
	if err != nil {
		return nil, cli.Errorf(cli.KindInvalid, "path cannot be opened")
	}
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(lstat, opened) {
		_ = root.Close()
		return nil, cli.Errorf(cli.KindInvalid, "path is a link or changed while it was opened")
	}
	return root, nil
}

// readSame reads the regular file p after checking, on the open descriptor,
// that it is the file Lstat saw.
func readSame(p string, lstat fs.FileInfo) ([]byte, error) {
	dir, name := filepath.Split(p)
	if dir == "" {
		dir = "."
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, cli.Errorf(cli.KindInvalid, "path cannot be opened")
	}
	defer func() { _ = root.Close() }()
	b, err := readRegular(root, name, lstat)
	if err != nil {
		var ce *cli.Error
		if errors.As(err, &ce) {
			return nil, err
		}
		return nil, cli.Errorf(cli.KindInvalid, "file cannot be read")
	}
	return b, nil
}
