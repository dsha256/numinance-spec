package cli

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

const (
	initFileMode = 0o644
	initDirMode  = 0o755
	// tmpSuffix names the file a document is written to before it is renamed into place.
	tmpSuffix = ".numi-init.tmp"
	// configRoot is the directory init creates and owns.
	configRoot = ".numinance"
)

// treeWriter writes the files of one tree below an open os.Root, so no path leaves the directory. It checks the whole
// tree before it writes anything, refuses a link on any path it uses, and never overwrites without force. Each file is
// written to a temporary name and renamed, so a reader never sees a half-written document; if a write fails, what
// this run created is removed again (a file replaced under force cannot be restored).
type treeWriter struct {
	root    *os.Root
	files   []outFile
	force   bool
	created []string
	// tool names the command in messages: "init" or "discover".
	tool string
}

// openTree opens dir and checks that the files can be written.
func openTree(dir string, files []outFile, force bool) (*treeWriter, error) {
	return openTreeFor("init", dir, files, force)
}

// openTreeFor is openTree for the command named tool, which its messages use.
func openTreeFor(tool, dir string, files []outFile, force bool) (*treeWriter, error) {
	root, err := os.OpenRoot(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, safeErrorf(KindNotFound, "directory %s not found", quote(dir))
	case err != nil:
		return nil, safeErrorf(KindInvalid, "%s is not a directory that can be opened", quote(dir))
	}
	w := &treeWriter{root: root, files: files, force: force, tool: tool}
	if err := w.preflight(); err != nil {
		_ = root.Close()
		return nil, err
	}
	return w, nil
}

// distinctFrom refuses two trees that are the same directory under different spellings (a dot, a relative path or a
// symlink), or whose config directories are the same directory: the org tree would otherwise overwrite the repository's
// documents, or the repository's, in one run. The roots are compared as opened, not as typed.
func (w *treeWriter) distinctFrom(o *treeWriter) error {
	a, errA := w.root.Stat(".")
	b, errB := o.root.Stat(".")
	if errA != nil || errB != nil {
		return Errorf(KindInternal, "cannot inspect the directories")
	}
	if os.SameFile(a, b) {
		return Errorf(KindInvalid, "--org-dir is the repository itself (same directory by another spelling); the organization policy repository is a different directory")
	}
	ca, errA := w.root.Stat(configRoot)
	cb, errB := o.root.Stat(configRoot)
	if errA == nil && errB == nil && os.SameFile(ca, cb) {
		return Errorf(KindInvalid, "--org-dir and the repository share one %s directory; the organization policy repository is a different directory", configRoot)
	}
	return nil
}

func (w *treeWriter) close() { _ = w.root.Close() }

// preflight is the whole check before the first byte is written.
func (w *treeWriter) preflight() error {
	if err := w.preflightOutside(); err != nil {
		return err
	}
	fi, err := w.root.Lstat(configRoot)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return Errorf(KindInternal, "cannot inspect %s", configRoot)
	case fi.Mode()&fs.ModeSymlink != 0:
		return Errorf(KindInvalid, "%s is a symbolic link; refusing to write through it", configRoot)
	case !fi.IsDir():
		return Errorf(KindInvalid, "%s exists and is not a directory", configRoot)
	}
	if !w.force {
		empty, err := w.empty(configRoot)
		if err != nil {
			return err
		}
		if !empty {
			return Errorf(KindConflict, "%s already has files; pass --force to overwrite the files %s writes (other files are never touched)", configRoot, w.tool)
		}
	}
	for _, f := range w.files {
		for _, d := range parentDirs(f.Path) {
			fi, err := w.root.Lstat(d)
			switch {
			case errors.Is(err, fs.ErrNotExist):
			case err != nil:
				return safeErrorf(KindInternal, "cannot inspect %s", quote(d))
			case !fi.IsDir():
				return safeErrorf(KindInvalid, "%s is a link or not a directory; refusing to write through it", quote(d))
			}
		}
		fi, err := w.root.Lstat(f.Path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return safeErrorf(KindInternal, "cannot inspect %s", quote(f.Path))
		case !fi.Mode().IsRegular():
			return safeErrorf(KindInvalid, "%s is a link or not a regular file; refusing to overwrite it", quote(f.Path))
		}
	}
	return nil
}

// preflightOutside checks the files that are not below configRoot (init has none; discover writes PULL_REQUEST.md
// beside it): a link or a special file is refused, and an existing file is a conflict without force.
func (w *treeWriter) preflightOutside() error {
	for _, f := range w.files {
		if strings.HasPrefix(f.Path, configRoot+"/") {
			continue
		}
		fi, err := w.root.Lstat(f.Path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return safeErrorf(KindInternal, "cannot inspect %s", quote(f.Path))
		case !fi.Mode().IsRegular():
			return safeErrorf(KindInvalid, "%s is a link or not a regular file; refusing to overwrite it", quote(f.Path))
		case !w.force:
			return safeErrorf(KindConflict, "%s already exists; pass --force to overwrite the files %s writes", quote(f.Path), w.tool)
		}
	}
	return nil
}

// empty reports whether the directory has no entry at all.
func (w *treeWriter) empty(dir string) (bool, error) {
	d, err := w.root.Open(dir)
	if err != nil {
		return false, Errorf(KindInternal, "cannot read %s", dir)
	}
	defer func() { _ = d.Close() }()
	_, err = d.ReadDir(1)
	switch {
	case errors.Is(err, io.EOF):
		return true, nil
	case err != nil:
		return false, Errorf(KindInternal, "cannot read %s", dir)
	}
	return false, nil
}

// parentDirs lists the directories above a slash path, outermost first.
func parentDirs(p string) []string {
	var dirs []string
	for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
		dirs = append(dirs, d)
	}
	slices.Reverse(dirs)
	return dirs
}

// write creates the directories and writes every file. On failure it undoes what it created.
func (w *treeWriter) write(ctx context.Context) error {
	if err := w.writeAll(ctx); err != nil {
		w.undo()
		return err
	}
	return nil
}

func (w *treeWriter) writeAll(ctx context.Context) error {
	for _, f := range w.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, d := range parentDirs(f.Path) {
			if err := w.mkdir(d); err != nil {
				return err
			}
		}
		if err := w.writeFile(f); err != nil {
			return err
		}
	}
	return nil
}

func (w *treeWriter) mkdir(d string) error {
	if _, err := w.root.Lstat(d); err == nil {
		return nil
	}
	if err := w.root.Mkdir(d, initDirMode); err != nil {
		return safeErrorf(KindInternal, "cannot create %s", quote(d))
	}
	w.created = append(w.created, d)
	// The mode is set again because the process umask may have cleared bits of it.
	if err := w.root.Chmod(d, initDirMode); err != nil {
		return safeErrorf(KindInternal, "cannot set the mode of %s", quote(d))
	}
	return nil
}

func (w *treeWriter) writeFile(f outFile) (err error) {
	_, statErr := w.root.Lstat(f.Path)
	existed := statErr == nil
	tmp := f.Path + tmpSuffix
	out, err := w.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, initFileMode)
	if errors.Is(err, fs.ErrExist) {
		return safeErrorf(KindConflict, "%s is left over from an earlier run; remove it", quote(tmp))
	}
	if err != nil {
		return safeErrorf(KindInternal, "cannot write %s", quote(f.Path))
	}
	defer func() {
		if err != nil {
			_ = w.root.Remove(tmp)
		}
	}()
	_, werr := out.Write(f.Data)
	cherr := out.Chmod(initFileMode)
	clerr := out.Close()
	if err := errors.Join(werr, cherr, clerr); err != nil {
		return safeErrorf(KindInternal, "cannot write %s", quote(f.Path))
	}
	if existed && !w.force {
		return safeErrorf(KindConflict, "%s appeared during the run; pass --force to overwrite it", quote(f.Path))
	}
	if existed || w.force {
		err = w.root.Rename(tmp, f.Path)
	} else {
		// Without force nothing may be replaced: a hard link fails if a file appeared since the preflight. Where the
		// file system has no hard links the rename is the fallback, and the window between check and rename stays.
		err = w.root.Link(tmp, f.Path)
		switch {
		case errors.Is(err, fs.ErrExist):
			return safeErrorf(KindConflict, "%s appeared during the run; pass --force to overwrite it", quote(f.Path))
		case err == nil:
			_ = w.root.Remove(tmp)
		default:
			err = w.root.Rename(tmp, f.Path)
		}
	}
	if err != nil {
		return safeErrorf(KindInternal, "cannot write %s", quote(f.Path))
	}
	if !existed {
		w.created = append(w.created, f.Path)
	}
	return nil
}

// undo removes what this run created, newest first; a directory that is not empty stays.
func (w *treeWriter) undo() {
	for _, p := range slices.Backward(w.created) {
		_ = w.root.Remove(p)
	}
	w.created = nil
}
