// pkg/merger/file_auth.go
package merger

import (
	"fmt"

	"github.com/intentd/inrun/pkg/util/fileio"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/types"
)

// loadImportFileWithAuth loads a Blueprint import file with optional authentication.
// Imports must be Blueprints — a Stack cannot import another Stack.
func (m *Merger) loadImportFileWithAuth(stackPath, importPath string, auth *fileio.FileAuth) (map[string]types.CRDEntry, error) {
	data, err := loadFileWithAuth(importPath, auth)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", importPath, err)
	}

	doc, err := parseBlueprintDoc(data, importPath)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		logger.Debug().
			Str("path", importPath).
			Msg("merger: skipping import — not a valid Blueprint document")
		return nil, nil
	}

	if doc.Kind == config.StackKind() {
		return nil, fmt.Errorf(
			"%q imports.files[%q]: a Stack cannot import another Stack — "+
				"only Blueprint files are valid imports",
			stackPath, importPath,
		)
	}

	if doc.Kind != config.BlueprintKind() {
		return nil, fmt.Errorf(
			"%q imports.files[%q]: expected kind %q, got %q",
			stackPath, importPath, config.BlueprintKind(), doc.Kind,
		)
	}

	return m.loadBlueprint(importPath, doc)
}
