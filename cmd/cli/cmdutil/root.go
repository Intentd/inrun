package cmdutil

import (
	"context"
	"os"
	"path/filepath"

	"github.com/intentd/inrun/pkg/util/term"

	"github.com/spf13/cobra"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
)

var (
	Cfg *config.Config
	Ctx context.Context
)

var RootCmd = &cobra.Command{
	Use:   "inrun",
	Short: "Inrun runs intents on Kubernetes",
	Long:  "Inrun runs intents on Kubernetes: callers send intents through the gateway,\nreconcilers turn them into resources, and the result comes back as a view.",
}

func Execute(k *config.Config, c context.Context) {
	Cfg = k
	Ctx = c

	if err := RootCmd.ExecuteContext(c); err != nil {
		term.Exit(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// SilenceUsage is an option to silence usage when an error occurs.
	RootCmd.SilenceUsage = true
	// SilenceErrors is an option to quiet errors down stream.
	RootCmd.SilenceErrors = true
	RootCmd.CompletionOptions.DisableDefaultCmd = true

	// Global flags — always present in both runtime and dev builds
	RootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	RootCmd.PersistentFlags().StringSliceP("file", "f", nil, "Path(s) or URL(s) to blueprint.yaml (repeatable)")
	// Dev-only flags (--kubeconfig, --verbose) and required-flag marking for
	// dev commands are registered in root_dev.go (//go:build !runtime).
}

func initConfig() {
	// Resolve log level (flag > env > default) and initialize logger
	level := resolveLogLevel(RootCmd)
	logger.Init(level)

	// Resolve kubeconfig path (flag > env > ~/.kube/config > in‑cluster)
	kubeconfig := resolveKubeconfig(RootCmd)

	// Persist resolved values into global Config
	if Cfg != nil {
		Cfg.Cluster().SetKubeconfigPath(kubeconfig)
	}
}

// resolveKubeconfig determines which kubeconfig to use.
// Priority: CLI flag → $KUBECONFIG → ~/.kube/config → in‑cluster.
func resolveKubeconfig(cmd *cobra.Command) string {
	if flagVal, _ := cmd.Flags().GetString("kubeconfig"); flagVal != "" {
		return flagVal
	}

	if envVal := Cfg.Cluster().KubeconfigPath(); envVal != "" {
		return envVal
	}
	if home, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath
		}
	}
	return ""
}

// resolveLogLevel determines the effective log level.
// Priority: --debug flag → LOG_LEVEL env → "info".
func resolveLogLevel(cmd *cobra.Command) string {
	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		return "debug"
	}

	if envVal := Cfg.Inrun().LogLevel(); envVal != "" {
		return envVal
	}

	return "info"
}
