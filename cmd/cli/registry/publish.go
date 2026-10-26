//go:build !runtime && !gateway

package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/registry"
)

// pushWithProgress wraps a registry upload with the standard CLI spinner.
// The callback owns the actual client and push options, keeping those details
// out of this presentation helper.
func pushWithProgress(label string, push func(progress func(file string, size int64)) (string, error)) (string, error) {
	spin := cmdutil.StartSpinner(label)
	progress := func(file string, size int64) {
		spin.Update(fmt.Sprintf("Uploading %s (%s)", file, cmdutil.FormatSize(size)))
	}

	digest, err := push(progress)
	if err != nil {
		spin.Failure()
		return "", err
	}
	spin.Stop()
	return digest, nil
}

// publishOptionalModule publishes the module alongside a blueprint when the
// directory contains module.yaml. A failed optional module upload remains a
// warning and does not undo the successful blueprint push.
func publishOptionalModule(
	dir string,
	publish func() (ref, digest, warning string, err error),
) {
	if _, err := os.Stat(filepath.Join(dir, registry.FileModule)); err != nil {
		return
	}

	ref, digest, warning, err := publish()
	if err != nil {
		if warning != "" {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", warning, err)
		}
		return
	}
	if ref == "" || digest == "" {
		return
	}
	fmt.Printf("%s Pushed module: %s\n", cmdutil.SuccessMark(), ref)
	if len(digest) > 19 {
		digest = digest[:19] + "..."
	}
	fmt.Printf("  Digest: %s\n", digest)
}

func printImportInstructions(ref string, isModule bool) {
	fmt.Printf("\nTo import in a Blueprint:\n")
	if isModule {
		fmt.Printf("  imports:\n")
		fmt.Printf("    - module: %s\n", ref)
		return
	}
	fmt.Printf("  imports:\n")
	fmt.Printf("    registry:\n")
	fmt.Printf("      - url: %s\n", ref)
}
