//go:build !runtime && !gateway

package scaffold

import (
	"bytes"
	"regexp"

	"github.com/intentd/inrun/pkg/types"
)

var (
	inrunAPIVersion = regexp.MustCompile(`(?m)^apiVersion:\s*inrun\.dev/v1\s*$`)
	topLevelKind    = regexp.MustCompile(`(?m)^kind:\s*(\w+)\s*$`)
)

// withSchemaLine starts an Inrun YAML file with the yaml-language-server
// comment that points editors at its kind's schema, so a scaffolded project
// gets completion, hover docs and typo checks with no setup. Other files,
// and files that already name a schema, are returned unchanged.
func withSchemaLine(data []byte) []byte {
	if bytes.Contains(data, []byte("yaml-language-server:")) || !inrunAPIVersion.Match(data) {
		return data
	}
	m := topLevelKind.FindSubmatch(data)
	if m == nil {
		return data
	}
	line := types.SchemaLine(string(m[1]))
	if line == "" {
		return data
	}
	return append([]byte(line+"\n"), data...)
}
