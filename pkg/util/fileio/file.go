package fileio

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileExists reports whether a file exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// CopyFile copies a file from src → dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := ReadLocal(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

// JoinRelative resolves p relative to the directory that contains base.
// If p is already absolute, it is returned unchanged.
func JoinRelative(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(filepath.Dir(base), p)
}

// ReadLocal reads a file from the local filesystem.
func ReadLocal(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file %q does not exist", path)
	}
	return os.ReadFile(path)
}

// ToAbs resolves path to an absolute path. If path is relative and baseDir
// is provided, it is resolved relative to baseDir.
func ToAbs(path, baseDir string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	if baseDir != "" {
		path = filepath.Join(baseDir, path)
	}

	return filepath.Abs(path)
}

// PathOrBaseMatches reports whether path matches pattern either as a
// component anywhere in the path, or as a glob against its base name.
func PathOrBaseMatches(pattern, path string) bool {
	base := filepath.Base(path)
	if ok, _ := filepath.Match(pattern, base); ok {
		return true
	}
	// Also match if any path component equals the pattern exactly.
	for _, part := range strings.Split(path, string(os.PathSeparator)) {
		if part == pattern {
			return true
		}
	}
	return false
}

// DiscoverFiles walks root, returning files whose name ends with suffix,
// skipping directories and files that match any of skip, and excluding
// files for which shouldExclude returns true. Results are sorted.
func DiscoverFiles(
	root string,
	suffix string,
	skip []string,
	shouldExclude func(path string) bool,
) ([]string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	var found []string
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			for _, pattern := range skip {
				if PathOrBaseMatches(pattern, path) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), suffix) {
			return nil
		}
		for _, pattern := range skip {
			if PathOrBaseMatches(pattern, path) {
				return nil
			}
		}
		if shouldExclude != nil && shouldExclude(path) {
			return nil
		}
		found = append(found, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(found)
	return found, nil
}

// IsDiscoveryArg reports whether arg explicitly requests recursive
// discovery using Go-style package-pattern syntax.
//
// Recognized patterns:
//
//	"./..."          → (".", true)
//	"..."            → (".", true)
//	"."              → (".", true)
//	"declarative/..." → ("declarative", true)
//
// Ordinary paths, including paths ending in "/", are not discovery patterns.
func IsDiscoveryArg(arg string) (root string, ok bool) {
	if arg == "." || arg == "..." {
		return ".", true
	}

	if strings.HasSuffix(arg, "/...") {
		root = strings.TrimSuffix(arg, "/...")
		if root == "" {
			root = "."
		}
		return root, true
	}

	return "", false
}

// RelFromDir returns path relative to dir (both resolved from the current
// directory), prefixed with ./ when it does not climb out of dir.
func RelFromDir(dir, path string) string {
	absDir, err1 := filepath.Abs(dir)
	absPath, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return path
	}
	r, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return path
	}
	if !strings.HasPrefix(r, "..") {
		r = "./" + r
	}
	return r
}

// TempFile creates a file named name inside a fresh temp directory and
// returns its path and a cleanup func. The cleanup removes the whole
// directory, so it must be deferred and called after the file is read.
func TempFile(name string) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "inrun-tmp-*")
	if err != nil {
		return "", nil, err
	}
	return filepath.Join(dir, name), func() { _ = os.RemoveAll(dir) }, nil
}
