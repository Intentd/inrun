// pkg/generate/helper.go
package generate

import (
	"embed"
	"fmt"

	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

func renderTemplateToFile(tmpl *template.Template, data any, outPath string, gofmt bool, dryRun bool) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("rendering template for %q: %w", outPath, err)
	}

	content := buf.Bytes()
	formatted, err := format.Source(content)
	if gofmt {
		if err != nil {
			return fmt.Errorf("gofmt failed for %q:\n%s\n\nerror: %w", outPath, buf.String(), err)
		}
		content = formatted
	}

	if dryRun {
		fmt.Println("--- dry run: would write to", filepath.Join(TypeRegistryPackage, RegistryFile), "---")
		fmt.Println(string(formatted))
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("creating directory %q: %w", filepath.Dir(outPath), err)
	}

	if err := os.WriteFile(outPath, content, 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", outPath, err)
	}

	return nil
}
