//go:build !runtime && !gateway

package self

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/intentd/inrun/cmd/cli/cmdutil"

	"github.com/intentd/inrun/pkg/version"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the Inrun CLI to the latest or a specific version",
	Long: `Upgrade the Inrun CLI (and optionally the Console) to the latest release.

Examples:
  inrun upgrade
  inrun upgrade --version v1.4.0
  inrun upgrade -v v1.4.0
  inrun upgrade --runtime-only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		check, _ := cmd.Flags().GetBool("check")
		requestedVersion, _ := cmd.Flags().GetString("version")
		runtimeOnly, _ := cmd.Flags().GetBool("runtime-only")

		if check {
			return runUpgradeCheck(requestedVersion)
		}

		return runUpgrade(requestedVersion, runtimeOnly)
	},
}

func init() {
	cmdutil.RootCmd.AddCommand(upgradeCmd)

	upgradeCmd.Flags().Bool("runtime-only", false, "Upgrade only the inrun runtime (skip inrun-console)")
	upgradeCmd.Flags().Bool("check", false, "Check if a newer Inrun version is available")

	// Shadow global flags so they don't appear under `inrun upgrade`
	cmdutil.ShadowGlobalCommandFlags(upgradeCmd, "file")
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Upgrade Logic
// ──────────────────────────────────────────────────────────────────────────────
//

func runUpgrade(requestedVersion string, runtimeOnly bool) error {
	cmdutil.PrintBanner()

	fmt.Printf("Upgrading Inrun CLI...\n")

	// Detect platform (linux_amd64, darwin_arm64, etc.)
	platform, err := detectPlatform()
	if err != nil {
		return err
	}

	// Resolve version (latest or user-specified)
	version, err := resolveVersion(requestedVersion)
	if err != nil {
		return err
	}

	fmt.Printf("→ Target version: %s\n", version)
	fmt.Printf("→ Platform: %s\n\n", platform)

	// Install inrun runtime
	if err := installBinary("inrun", platform, version); err != nil {
		return err
	}

	// Install inrun-console unless runtime-only
	if !runtimeOnly {
		_ = installBinary("inrun-console", platform, version)
	}

	fmt.Println()
	fmt.Println("✔ Upgrade complete")
	fmt.Println()

	// Print version
	fmt.Println("Current version:")
	fmt.Printf("  %s", cmdutil.Green(version))
	fmt.Println()

	return nil
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Platform Detection
// ──────────────────────────────────────────────────────────────────────────────
//

func detectPlatform() (string, error) {
	var osName, arch string

	switch runtime.GOOS {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "darwin"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	return fmt.Sprintf("%s_%s", osName, arch), nil
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Version Resolution (latest or pinned)
// ──────────────────────────────────────────────────────────────────────────────
//

func resolveVersion(requested string) (string, error) {
	if requested != "" {
		return requested, nil
	}

	// Fetch latest release tag from GitHub API
	resp, err := http.Get("https://api.github.com/repos/intentd/inrun/releases/latest")
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		TagName string `json:"tag_name"`
	}

	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &data)

	if data.TagName == "" {
		return "", fmt.Errorf("could not resolve latest version")
	}

	return data.TagName, nil
}

// should find a way to use requested
// Not a concern for now
func runUpgradeCheck(requested string) error {
	cmdutil.PrintBanner()

	fmt.Printf("Checking for Inrun CLI updates...\n")

	// Current version from ldflags
	current := version.Version

	// Resolve latest version
	latest, err := resolveVersion("") // "" fetches the latest
	if err != nil {
		return err
	}

	fmt.Printf("Current version: %s\n", current)
	fmt.Printf("Latest version:  %s\n\n", latest)

	if current == latest {
		fmt.Println("✔ You are already running the latest version.")
		return nil
	}

	fmt.Println("⚠ A new version is available!")
	fmt.Println("Run:")
	fmt.Println("  inrun upgrade")
	fmt.Println()

	return nil
}

//
// ──────────────────────────────────────────────────────────────────────────────
//  Install a Single Binary (inrun or inrun-console)
// ──────────────────────────────────────────────────────────────────────────────
//

func installBinary(binary, platform, ver string) error {
	// Install alongside the currently running binary so os.Rename stays on the
	// same filesystem (avoids "invalid cross-device link" on Linux).
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine install location: %w", err)
	}
	installDir := filepath.Dir(selfPath)
	installPath := filepath.Join(installDir, binary)

	archive := fmt.Sprintf("%s_%s.tar.gz", binary, platform)
	url := fmt.Sprintf("https://github.com/intentd/inrun/releases/download/%s/%s", ver, archive)

	fmt.Printf("→ Downloading %s...\n", archive)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  ⚠ %s not available for this version — skipping\n", binary)
		return nil
	}

	// Extract directly into a temp file in the install directory so
	// the final os.Rename is atomic and stays on the same filesystem.
	tmp, err := os.CreateTemp(installDir, "."+binary+".tmp.*")
	if err != nil {
		return fmt.Errorf("could not create temp file in %s: %w", installDir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	fmt.Printf("→ Extracting %s to %s...\n", archive, installPath)

	if err := cmdutil.ExtractBinaryFromTarGz(resp.Body, binary, tmp); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, installPath); err != nil {
		return fmt.Errorf("failed to install %s: %w", binary, err)
	}

	fmt.Printf("✔ Installed %s %s → %s\n", binary, ver, installPath)
	return nil
}
