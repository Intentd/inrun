package types

// SchemaBaseURL is where the editor schemas hack/generate-schema writes are
// served.
const SchemaBaseURL = "https://inrun.dev/schema/v1/"

// SchemaFiles maps each Inrun kind to its editor schema under SchemaBaseURL.
var SchemaFiles = map[string]string{
	"Blueprint": "blueprint.json",
	"Stack":     "stack.json",
	"Simulate":  "simulate.json",
	"E2E":       "e2e.json",
	"Module":    "module.json",
}

// SchemaLine returns the yaml-language-server comment that points editors at
// kind's schema, for completion, hover docs and typo checks; "" when kind has
// no schema. The line has no trailing newline.
func SchemaLine(kind string) string {
	file, ok := SchemaFiles[kind]
	if !ok {
		return ""
	}
	return "# yaml-language-server: $schema=" + SchemaBaseURL + file
}
