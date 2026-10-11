//go:build !runtime && !gateway

package generate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// splitLocationVersion splits "path" into modulePath and version.
// Accepts either "github.com/org/repo/pkg" or "github.com/org/repo/pkg@v1.2.3".
// Returns modulePath (no @) and version (empty if none).
func splitLocationVersion(path string) (modulePath, version string) {
	parts := strings.SplitN(strings.TrimSpace(path), "@", 2)
	modulePath = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		version = strings.TrimSpace(parts[1])
	}
	return
}

// collectModulesToGet inspects enabled CRDs and returns unique module@version strings
// for any HookDeclaration or ConstructorDeclaration that has Version set and Fetch == true.
// It tolerates version present in location (location@vX) by normalizing via splitLocationVersion.
func collectModulesToGet(crds map[string]types.CRDEntry) []string {
	seen := map[string]struct{}{}
	var mods []string

	for _, crd := range crds {
		if !crd.HasAnyHookTemplates() {
			continue
		}

		// --- Hooks ---
		if crd.CustomHooksEnabled() {
			h := crd.Box().Reconcile.Hooks
			// normalize if user put @version in location
			loc, locVer := splitLocationVersion(h.Location)
			version := h.Version
			if version == "" {
				version = locVer
				h.Version = locVer
			}

			// Update location
			h.Location = loc

			if version != "" && h.Fetch {
				modAt := loc + "@" + version
				if _, ok := seen[modAt]; !ok {
					seen[modAt] = struct{}{}
					mods = append(mods, modAt)
				}
			}
		}

		// --- Constructor ---
		if crd.ConstructorEnabled() {
			c := crd.Box().Reconcile.ConstructorDecl
			loc, locVer := splitLocationVersion(c.Location)
			version := c.Version

			if version != "" && version != locVer {
				fmt.Fprintf(os.Stderr, "[Warning] declared version %q differs from version in location %q; using declared version %q\n", version, locVer, version)
			}

			if version == "" {
				version = locVer
				c.Version = locVer
			}

			// Update location
			c.Location = loc

			if version != "" && c.Fetch {
				modAt := loc + "@" + version
				if _, ok := seen[modAt]; !ok {
					seen[modAt] = struct{}{}
					mods = append(mods, modAt)
				}
			}
		}
	}

	return mods
}

// goGetModules runs `go get module@version` for each module in mods.
// timeout is a per-module timeout. If dryRun is true, it only prints what would be run.
func goGetModules(mods []string, timeoutPerModule time.Duration, dryRun bool) error {
	if len(mods) == 0 {
		return nil
	}

	fmt.Printf("best-effort fetching %d module(s) referenced by catalog", len(mods))

	// ensure 'go' exists
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("\n'go' binary not found in PATH: %w \nRequired if 'fetch=true'", err)
	}

	for _, mod := range mods {
		if dryRun {
			fmt.Printf("[dry-run] would run: go get %s", mod)
			continue
		}

		// create a per-module context and cancel it immediately after Run
		ctx, cancel := context.WithTimeout(context.Background(), timeoutPerModule)
		defer cancel()

		cmd := exec.CommandContext(ctx, "go", "get", mod)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// inherit environment so GOPRIVATE/GONOSUMDB etc work if user set them
		cmd.Env = os.Environ()

		fmt.Printf("running: go get %s", mod)
		if err := cmd.Run(); err != nil {

			msg := fmt.Sprintf(
				"\nfailed to 'go get %q': %q\n"+
					"Hint: if this is a private module, ensure GOPRIVATE is set and credentials are available.\n"+
					"You can also run 'go get %q' manually", mod, err, mod,
			)
			return fmt.Errorf("%s", msg)
		}
		fmt.Printf("go get %s: OK", mod)
	}

	return nil
}
