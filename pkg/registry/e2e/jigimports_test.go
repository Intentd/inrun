package e2e

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Inrun talks to jig through its root package only: jig's packages are its
// own.
func TestOnlyJigRootIsImported(t *testing.T) {
	const jig = "github.com/intentd/jig"
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch {
			case d.Name() == ".git", d.Name() == "node_modules", d.Name() == "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(p, jig+"/") {
				t.Errorf("%s imports %s: use %s", path, p, jig)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
