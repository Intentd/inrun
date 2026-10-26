package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intentd/inrun/pkg/registry/simulate"
)

// Write writes the result to the configured report file.
// The output format is determined by the file extension: JSON for .json,
// Markdown otherwise.
func Write(file string, result *simulate.Result) error {
	if file == "" {
		return nil
	}

	var (
		data []byte
		err  error
	)

	switch strings.ToLower(filepath.Ext(file)) {
	case ".json":
		data, err = jSON(result)
	default:
		data = []byte(markdown(result))
	}

	if err != nil {
		return fmt.Errorf("encoding report: %w", err)
	}

	if err := os.WriteFile(file, data, 0644); err != nil {
		return fmt.Errorf("writing report file: %w", err)
	}

	fmt.Printf("  Report written to %s\n", file)
	return nil
}
