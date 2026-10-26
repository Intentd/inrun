// pkg/generate/pattern_generator.go
package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// WriteSimulateScaffold writes a simulate.yaml starter to dest.
func WriteSimulateScaffold(dest string) error {
	return writeStaticTemplate("templates/pattern_simulate.tmpl", dest)
}

// WriteE2EScaffold writes an e2e.yaml starter to dest.
// When typed is true, a valuesFiles entry referencing values.yaml is included.
func WriteE2EScaffold(dest string, typed bool) error {
	tmpl, err := template.ParseFS(templateFS, "templates/pattern_e2e.tmpl")
	if err != nil {
		return fmt.Errorf("parsing e2e template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, patternReadmeData{Typed: typed}); err != nil {
		return fmt.Errorf("rendering e2e: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", dest, err)
	}
	return os.WriteFile(dest, buf.Bytes(), 0o644)
}

// WriteValuesYAML writes a values.yaml with the runtime image placeholder to dest.
func WriteValuesYAML(dest string) error {
	return writeStaticTemplate("templates/pattern_values.tmpl", dest)
}

// WriteMakefile writes a clean Makefile (no example-pack workarounds) to dest.
func WriteMakefile(dest string) error {
	return writeStaticTemplate("templates/pattern_makefile.tmpl", dest)
}

// WriteDockerfile writes the production Dockerfile to dest.
func WriteDockerfile(dest string) error {
	return writeStaticTemplate("templates/pattern_dockerfile.tmpl", dest)
}

type patternReadmeData struct {
	Typed bool
}

// WriteREADME writes a README.md with actionable steps to dest.
// When typed is true, the steps include make registry, build, and release.
func WriteREADME(dest string, typed bool) error {
	tmpl, err := template.ParseFS(templateFS, "templates/pattern_readme.tmpl")
	if err != nil {
		return fmt.Errorf("parsing readme template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, patternReadmeData{Typed: typed}); err != nil {
		return fmt.Errorf("rendering readme: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", dest, err)
	}
	return os.WriteFile(dest, buf.Bytes(), 0o644)
}

// writeStaticTemplate reads a template file from the embedded FS and writes it
// to dest as-is — no template execution, raw bytes only.
func writeStaticTemplate(name, dest string) error {
	content, err := templateFS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("reading embedded %q: %w", name, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", dest, err)
	}
	return os.WriteFile(dest, content, 0o644)
}
