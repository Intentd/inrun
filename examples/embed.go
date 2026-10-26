//go:build !runtime

package examples

import "embed"

// FS holds the example packs embedded in the CLI; the runtime binary excludes it.
//
//go:embed reconcilers intent migration registry
var FS embed.FS
