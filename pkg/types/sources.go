// pkg/types/sources.go
package types

import (
	"fmt"
	"os"
	"strings"

	"github.com/intentd/inrun/pkg/utils"
)

// FileSource represents one file source entry.
// Supports both simple string form (just a path) and authenticated form.
//
// YAML unmarshalling handles both forms transparently via UnmarshalYAML.
// Users can write either:
//
//	files:
//	  - ./simple/path.yaml               # simple form
//	  - url: https://private/catalog.yaml # authenticated form
//	    auth:
//	      type: bearer
//	      fromEnv: MY_TOKEN
type FileSource struct {
	// URL or path — the file to load.
	// May be a local path, an HTTP(S) URL, or an $ENV_VAR reference.
	URL string `yaml:"url"`

	// Auth — optional authentication for remote sources.
	// Ignored for local paths.
	Auth *FileSourceAuth `yaml:"auth,omitempty"`
}

// FileSourceAuth declares how to authenticate when fetching a remote source.
// All credential values are resolved from environment variables at load time —
// credentials never appear as literal values in the Catalog YAML.
type FileSourceAuth struct {
	// Type — authentication scheme.
	// Supported values: "bearer", "github", "basic"
	Type string `yaml:"type" validate:"required,oneof=bearer github basic"`

	// FromEnv — environment variable containing the bearer token or GitHub token.
	// Used when Type is "bearer" or "github".
	FromEnv string `yaml:"fromEnv,omitempty"`

	// UsernameFromEnv — environment variable containing the username.
	// Used when Type is "basic".
	UsernameFromEnv string `yaml:"usernameFromEnv,omitempty"`

	// PasswordFromEnv — environment variable containing the password.
	// Used when Type is "basic".
	PasswordFromEnv string `yaml:"passwordFromEnv,omitempty"`
}

// UnmarshalYAML allows FileSource to be declared as either a plain string
// or a struct with url and auth fields.
//
// Plain string:
//   - ./path/to/catalog.yaml
//   - https://public.url/catalog.yaml
//   - $ENV_VAR
//
// Struct with auth:
//   - url: https://private.url/catalog.yaml
//     auth:
//     type: github
//     fromEnv: GITHUB_TOKEN
func (f *FileSource) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try plain string first
	var plain string
	if err := unmarshal(&plain); err == nil {
		f.URL = plain
		return nil
	}

	// Fall back to struct form
	type fileSourceAlias FileSource
	var alias fileSourceAlias
	if err := unmarshal(&alias); err != nil {
		return err
	}
	*f = FileSource(alias)
	return nil
}

// Resolve resolves the FileSourceAuth to a utils.FileAuth.
// Reads environment variables and returns the resolved credentials.
// Returns nil if the auth block is nil (unauthenticated source).
func (a *FileSourceAuth) Resolve() (*utils.FileAuth, error) {
	if a == nil {
		return nil, nil
	}

	auth := &utils.FileAuth{
		Type: a.Type,
	}

	switch strings.ToLower(a.Type) {
	case "bearer", "github":
		if a.FromEnv == "" {
			return nil, fmt.Errorf("auth type %q requires fromEnv", a.Type)
		}
		token := os.Getenv(a.FromEnv)
		if token == "" {
			return nil, fmt.Errorf(
				"auth type %q: environment variable %q is not set or empty",
				a.Type, a.FromEnv,
			)
		}
		auth.BearerToken = token

	case "basic":
		if a.UsernameFromEnv == "" {
			return nil, fmt.Errorf("basic auth requires usernameFromEnv")
		}
		auth.Username = os.Getenv(a.UsernameFromEnv)
		if auth.Username == "" {
			return nil, fmt.Errorf(
				"basic auth: environment variable %q is not set or empty",
				a.UsernameFromEnv,
			)
		}
		if a.PasswordFromEnv != "" {
			auth.Password = os.Getenv(a.PasswordFromEnv)
		}
	}

	return auth, nil
}
