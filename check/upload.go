package check

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"llocal.app/kit/recipe"
)

// upload copies into dst what llocal takes in of the project in dir: the files git would
// commit (outside a git repository, every file), less the folders an upload leaves out,
// like node_modules and build output. A link fails, as it fails an upload.
func upload(dir, dst string) error {
	files, err := projectFiles(dir)
	if err != nil {
		return err
	}
	for _, p := range files {
		parts := strings.Split(p, "/")
		if p == "" || slices.ContainsFunc(parts[:len(parts)-1], func(d string) bool { return slices.Contains(recipe.SkipDirs, d) }) {
			continue
		}
		fi, err := os.Lstat(filepath.Join(dir, p))
		switch {
		case errors.Is(err, fs.ErrNotExist) || err == nil && fi.IsDir():
			continue // deleted since git last saw it, or a submodule
		case err != nil:
			return err
		case fi.Mode()&fs.ModeSymlink != 0:
			return fmt.Errorf("%s is a link: llocal takes only files and folders, so put the file itself there", p)
		case !fi.Mode().IsRegular():
			return fmt.Errorf("%s is neither a file nor a folder: llocal takes only files and folders", p)
		}
		if err := copyFile(filepath.Join(dir, p), filepath.Join(dst, p), fi.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

// projectFiles are the paths in dir of the files git would commit, or outside a git
// repository every file but those in the folders an upload leaves out.
func projectFiles(dir string) ([]string, error) {
	out, err := exec.Command("git", "-C", dir, "ls-files", "-z", "--deduplicate", "--cached", "--others", "--exclude-standard").Output()
	if err == nil {
		return strings.Split(string(out), "\x00"), nil
	}
	var files []string
	err = fs.WalkDir(os.DirFS(dir), ".", func(p string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && p != "." && slices.Contains(recipe.SkipDirs, d.Name()):
			return fs.SkipDir
		case !d.IsDir():
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}
