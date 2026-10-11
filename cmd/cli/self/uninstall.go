//go:build !runtime && !gateway

package self

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/spf13/cobra"
)

//
// ──────────────────────────────────────────────────────────────────────────────
//  Command: inrun uninstall
//  Removes Inrun binaries, completions, and local cache.
// ──────────────────────────────────────────────────────────────────────────────
//

func newUninstallCmd() *cobra.Command {
	var yes bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Inrun CLI and remove all related files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(yes, dryRun)
		},
	}

	// Local flags
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Do not prompt for confirmation")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be removed without deleting anything")

	return cmd
}

// runUninstall executes the uninstall logic.
// Supports --dry-run and --yes for non-interactive removal.
func runUninstall(yes, dryRun bool) error {
	usr, _ := user.Current()
	home := usr.HomeDir

	// Resolve install directory — matches what install.sh uses by default.
	installDir := filepath.Join(home, ".inrun", "bin")
	if v := os.Getenv("INRUN_INSTALL_DIR"); v != "" {
		installDir = v
	}

	// All paths that may be removed
	paths := []string{
		// ~/.inrun covers the binaries, config, cache, and plugins.
		// Listed explicitly so dry-run shows what will go.
		filepath.Join(installDir, "inrun"),
		filepath.Join(installDir, "inrun-console"),

		filepath.Join(home, ".bash_completion.d/inrun"),
		filepath.Join(home, ".zsh/completions/_ork"),
		filepath.Join(home, ".config/fish/completions/inrun.fish"),

		filepath.Join(home, ".inrun"),
	}

	// Dry-run: show what would be removed
	if dryRun {
		fmt.Printf("\nThis is a dry run. No files will be removed.\n")
		fmt.Println("Would remove:")
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				fmt.Printf("  %s\n", p)
			}
		}
		fmt.Println("\n✓ Dry run complete")
		return nil
	}

	// Confirmation prompt (unless --yes)
	if !yes {
		fmt.Print("This will remove Inrun, Inrun Console, cache, and completions. Continue? [y/N]: ")
		var resp string
		fmt.Scanln(&resp)
		if resp != "y" && resp != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("\nUninstalling Inrun...")

	// Remove all known paths
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			fmt.Printf("  Removing %s\n", p)
			os.RemoveAll(p)
		}
	}

	fmt.Println("\n✓ Inrun uninstalled successfully")
	return nil
}

// Register uninstall command and shadow global flags so they don't appear here.
func init() {
	uninstallCmd := newUninstallCmd()
	cmdutil.RootCmd.AddCommand(uninstallCmd)

	// Shadow global flags (so they don't show under `inrun uninstall`)
	cmdutil.ShadowGlobalCommandFlags(uninstallCmd, "file")
}
