package check

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"llocal.app/kit/recipe"
)

const maxHops = 40

var (
	// maxCopy bounds what the links of one build may copy in.
	maxCopy  int64 = 1 << 30
	absolute       = errors.New("an absolute path: llocal copies in what a link points to only inside the app, by a relative path, so put the file itself there")
	outside        = errors.New("outside the app: llocal copies in what a link points to only inside the app, so put the file itself there")
	circle         = errors.New("which leads round in a circle: remove the link")
)

// Finish does to an app's folder, once built, what llocal does before it keeps a build: a
// standard app's node_modules folders, which only build its front end, go; each link in
// what llocal keeps (the whole app, or a static app's static folder) becomes what it points
// to, a copy, or for a command in node_modules/.bin a script that runs it; and a link that
// points outside the app, by an absolute path, to nothing or round in a circle fails, as
// does a file that is neither a file nor a folder. It changes dir.
func Finish(dir string) error {
	rec, err := recipe.Load(filepath.Join(dir, "llocal.json"))
	if err != nil {
		return err
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	kept := "."
	if rec.Shape == "static" {
		kept = rec.Static
	}
	var links, drop []string
	err = fs.WalkDir(root.FS(), kept, func(p string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist) && p == kept:
			return fs.SkipAll // no static folder: llocal says so when it copies it
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "node_modules" && rec.Shape == "standard":
			drop = append(drop, p)
			return fs.SkipDir
		case d.Type() == fs.ModeSymlink:
			links = append(links, p)
		case !d.IsDir() && !d.Type().IsRegular():
			return special(p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	f := &finisher{dir: dir, root: root, budget: maxCopy}
	for _, p := range links {
		if err := f.link(p, p, []string{path.Dir(p)}); err != nil {
			return err
		}
	}
	for _, p := range drop {
		if err := os.RemoveAll(filepath.Join(dir, p)); err != nil {
			return err
		}
	}
	return nil
}

type finisher struct {
	dir    string
	root   *os.Root
	budget int64
}

// link puts what the link src points to at dst: in its place when dst is src, or in a copy
// of a folder that holds it. within are the folders, in the app, being copied into dst.
func (f *finisher) link(src, dst string, within []string) error {
	to, err := os.Readlink(filepath.Join(f.dir, src))
	if err != nil {
		return err
	}
	real, err := f.real(src)
	if err == nil && dst == src {
		err = f.root.Remove(src)
	}
	switch {
	case errors.Is(err, absolute), errors.Is(err, outside), errors.Is(err, circle):
		return fmt.Errorf("%s is a link to %s, %w", src, to, err)
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("%s is a link to %s, which isn't there: remove the link, or have the build make what it points to", src, to)
	case err != nil:
		return err
	}
	fi, err := f.root.Lstat(real)
	switch {
	case err != nil:
		return err
	case fi.IsDir() && (real == "." || slices.ContainsFunc(within, func(d string) bool { return strings.HasPrefix(d+"/", real+"/") })):
		return fmt.Errorf("%s is a link to %s, a folder that holds the link, so a copy would never end: remove the link", src, to)
	case fi.IsDir():
		return f.folder(real, dst, append(within, real))
	case !fi.Mode().IsRegular():
		return fmt.Errorf("%s is a link to %s, which is neither a file nor a folder: remove the link", src, to)
	case path.Base(path.Dir(dst)) == ".bin" && path.Base(path.Dir(path.Dir(dst))) == "node_modules":
		// A command runs its file where it is, as the link did, so the file finds its own.
		return f.write(dst, 0o755, strings.NewReader(fmt.Sprintf("#!/bin/sh\nexec '%s' \"$@\"\n", strings.ReplaceAll(filepath.Join(f.dir, real), "'", `'\''`))))
	}
	return f.copy(real, dst, fi)
}

// real is where p, a path in the app that may pass through links, leads: a path in the app
// through no link.
func (f *finisher) real(p string) (string, error) {
	cur, rest, hops := ".", strings.Split(p, "/"), 0
	for len(rest) > 0 {
		part := rest[0]
		rest = rest[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if cur == "." {
				return "", outside
			}
			cur = path.Dir(cur)
			continue
		}
		next := path.Join(cur, part)
		fi, err := f.root.Lstat(next)
		if err != nil {
			return "", err
		}
		if fi.Mode()&fs.ModeSymlink == 0 {
			cur = next
			continue
		}
		if hops++; hops > maxHops {
			return "", circle
		}
		to, err := os.Readlink(filepath.Join(f.dir, next))
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(to) {
			return "", absolute
		}
		rest = append(strings.Split(to, "/"), rest...)
	}
	return cur, nil
}

// folder copies the folder src into dst, which doesn't exist yet, putting what links point
// to in their place.
func (f *finisher) folder(src, dst string, within []string) error {
	if err := f.root.Mkdir(dst, 0o755); err != nil {
		return err
	}
	entries, err := fs.ReadDir(f.root.FS(), src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from, to := path.Join(src, e.Name()), path.Join(dst, e.Name())
		switch {
		case e.Type() == fs.ModeSymlink:
			err = f.link(from, to, within)
		case e.IsDir():
			err = f.folder(from, to, within)
		case e.Type().IsRegular():
			var fi fs.FileInfo
			if fi, err = e.Info(); err == nil {
				err = f.copy(from, to, fi)
			}
		default:
			err = special(from)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func special(p string) error {
	return fmt.Errorf("%s is neither a file nor a folder: llocal keeps only files and folders, so have the build remove it", p)
}

func (f *finisher) copy(src, dst string, fi fs.FileInfo) error {
	if f.budget -= fi.Size(); f.budget < 0 {
		return fmt.Errorf("the links the build left point to more than %d MB in all, more than llocal copies in their place: remove the links the app doesn't need", maxCopy>>20)
	}
	in, err := f.root.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return f.write(dst, fi.Mode().Perm(), in)
}

func (f *finisher) write(dst string, mode fs.FileMode, r io.Reader) error {
	out, err := f.root.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	return errors.Join(err, out.Close())
}
