//go:build !runtime && !gateway

// Package console implements the console command, which starts the console
// web UI as a separate process.
package console

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/util/version"

	"github.com/spf13/cobra"
)

var (
	consolePort          string
	consoleURLs          string
	consoleRefresh       string
	consoleLogLevel      string
	consoleIgnoreDefault bool
)

func init() {
	controlCmd.Flags().StringVarP(&consolePort, "port", "p", "8081", "Port to run the Console on")
	controlCmd.Flags().BoolVarP(&consoleIgnoreDefault, "ignore-default", "i", false, "Do not add the default localhost:8080 URL; start with no instances")
	controlCmd.Flags().StringVarP(&consoleURLs, "urls", "u", "http://localhost:8080", "Comma-separated list of Inrun runtime URLs")
	controlCmd.Flags().StringVar(&consoleRefresh, "refresh", "10s", "Refresh interval for fetching Blueprints")
	controlCmd.Flags().StringVar(&consoleLogLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	controlCmd.AddCommand(controlVersionCmd)
	cmdutil.RootCmd.AddCommand(controlCmd)

	// Shadow global flags so they don't appear under `inrun console`
	controlCmd.Flags().StringSlice("blueprint", nil, "")
	controlCmd.Flags().MarkHidden("blueprint")
	cmdutil.ShadowGlobalCommandFlags(controlCmd)
}

var controlCmd = &cobra.Command{
	Use:   "console",
	Short: "Start the Inrun Console",
	Long: `Start the Inrun Console web UI.

The Console provides a web-based interface for monitoring
multiple Inrun runtime and gateway instances.

Examples:
  # Start with default settings (port 8081, localhost:8080)
  inrun console

  # Start on custom port with multiple instances
  inrun console --port 9090 --urls "http://localhost:8080,http://localhost:8082"

  # Start with debug logging
  inrun console --log-level debug --refresh 5s

  # Monitor remote instances
  inrun console --urls "https://inrun.prod.internal:8080,https://inrun.staging:8080"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return startConsole()
	},
}

var controlVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Console version",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showConsoleVersion()
	},
}

func startConsole() error {
	consolePath, err := FindBinary()
	if err != nil {
		fmt.Fprintln(os.Stderr, "[inrun] inrun-console not found, attempting installation...")
		if err := installConsoleBinary(); err != nil {
			return fmt.Errorf("failed to install inrun-console: %w", err)
		}
		consolePath, err = FindBinary()
		if err != nil {
			return fmt.Errorf("inrun-console still not found after installation")
		}
	}

	fmt.Printf("[inrun] Starting Console: %s -p %s -u %s\n", consolePath, consolePort, consoleURLs)

	// Direct flags, no "start" subcommand
	args := []string{
		"-p", consolePort,
		"-u", consoleURLs,
		"--refresh", consoleRefresh,
		"--log-level", consoleLogLevel,
	}

	cmd := exec.Command(consolePath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Ctrl+C reaches the console directly (same process group); wait for it
	// to shut down instead of exiting first. Pass SIGTERM on.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting console: %w", err)
	}
	go func() {
		for s := range sigs {
			if s == syscall.SIGTERM {
				_ = cmd.Process.Signal(s)
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("console exited with error: %w", err)
	}

	return nil
}

func showConsoleVersion() error {
	consolePath, err := FindBinary()
	if err != nil {
		return fmt.Errorf("inrun-console not found: %w", err)
	}

	cmd := exec.Command(consolePath, "--version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// FindBinary locates the inrun-console binary: PATH, next to inrun, then
// ~/.inrun/bin.
func FindBinary() (string, error) {
	// Look in PATH first
	if path, err := exec.LookPath("inrun-console"); err == nil {
		return path, nil
	}

	// Look next to the inrun binary
	inrunPath, err := exec.LookPath("inrun")
	if err == nil {
		dir := strings.TrimSuffix(inrunPath, "/inrun")
		candidate := dir + "/inrun-console"
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Look in ~/.inrun/bin
	home, err := os.UserHomeDir()
	if err == nil {
		candidate := home + "/.inrun/bin/inrun-console"
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("inrun-console not found in PATH, ~/.inrun/bin, or next to inrun binary")
}

func installConsoleBinary() error {
	ver := version.Short()
	if !strings.HasPrefix(ver, "v") {
		return fmt.Errorf("inrun-console not found — build it manually: make inrun-console")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binDir := home + "/.inrun/bin"
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	goos := runtime.GOOS
	platform := goos + "_" + runtime.GOARCH
	archive := fmt.Sprintf("inrun-console_%s.tar.gz", platform)
	url := fmt.Sprintf("https://github.com/intentd/inrun/releases/download/%s/%s", ver, archive)
	dest := binDir + "/inrun-console"
	if goos == "windows" {
		dest += ".exe"
	}

	fmt.Printf("[inrun] Downloading %s...\n", archive)
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("inrun-console %s not found in release %s (HTTP %d) — build manually: make inrun-console", platform, ver, resp.StatusCode)
	}

	tmp, err := os.CreateTemp(binDir, ".inrun-console.tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := cmdutil.ExtractBinaryFromTarGz(resp.Body, "inrun-console", tmp); err != nil {
		tmp.Close()
		return fmt.Errorf("extract failed: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return err
	}
	fmt.Printf("[inrun] inrun-console installed to %s\n", dest)
	return nil
}
