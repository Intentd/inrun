//go:build !runtime && !gateway

package companions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/intentd/inrun/cmd/cli/console"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
)

// Start starts the console next to the runtime, and the gateway too when the
// Catalog serves (gateway API enabled and a CRD with serve enabled). Each runs
// as a child process, so one that fails never takes the runtime down: a
// companion that cannot start is logged and skipped. A signal that stops inrun
// is passed on, so the children stop with it.
//
// paths are the Catalogs the gateway loads; cfg is the runtime's config, which
// advertises the gateway to the console.
func Start(ctx context.Context, cfg *config.Config, paths []string, cat *catalog.Catalog) {
	var children []*exec.Cmd

	if cat.HasServeEnabled() {
		cmd, url, err := startGateway(ctx, paths)
		if err != nil {
			logger.Warn().Err(err).Msg("gateway not started")
		} else {
			children = append(children, cmd)
			// The runtime's /catalog advertises it, so the console finds it.
			cfg.SetGatewayEndpoint(url)
		}
	}

	// The console finds the gateway through the runtime's /catalog.
	cmd, err := startConsole(ctx, "http://localhost:"+cfg.Health().Port())
	if err != nil {
		logger.Warn().Err(err).Msg("console not started")
	} else {
		children = append(children, cmd)
	}

	stopWithInrun(children)
}

// stopWithInrun stops the children when inrun gets SIGINT or SIGTERM. Ctrl+C
// in a terminal already reaches them; a signal sent to inrun alone would not.
//
// Children get SIGTERM, which they cannot have inherited as ignored. The
// signal is then raised again on inrun with this handler removed, so inrun
// reacts exactly as it would without it: a graceful shutdown once the runtime
// has started, the default action before that.
func stopWithInrun(children []*exec.Cmd) {
	if len(children) == 0 {
		return
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigs
		for _, c := range children {
			_ = c.Process.Signal(syscall.SIGTERM)
		}
		signal.Stop(sigs)
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			_ = p.Signal(s)
		}
	}()
}

// startGateway runs this binary's `gate run` with the same Catalogs, on
// INRUN_GATEWAY_PORT (default 8082).
func startGateway(ctx context.Context, paths []string) (*exec.Cmd, string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, "", fmt.Errorf("locating inrun: %w", err)
	}
	port := config.GetStrEnv("INRUN_GATEWAY_PORT", "8082")
	args := []string{"gate", "run"}
	for _, p := range paths {
		args = append(args, "-f", p)
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), "INRUN_PORT="+port)
	if err := start(cmd, "gateway"); err != nil {
		return nil, "", err
	}
	url := "http://localhost:" + port
	logger.Info().Str("url", url).Msg("gateway started")
	return cmd, url, nil
}

// startConsole runs inrun-console on INRUN_CONSOLE_PORT (default 8081),
// watching the runtime.
func startConsole(ctx context.Context, runtimeURL string) (*exec.Cmd, error) {
	bin, err := console.FindBinary()
	if err != nil {
		return nil, err
	}
	port := config.GetStrEnv("INRUN_CONSOLE_PORT", "8081")
	cmd := exec.CommandContext(ctx, bin,
		"-p", port,
		"-u", runtimeURL,
		"--log-level", "warn",
	)
	if err := start(cmd, "console"); err != nil {
		return nil, err
	}
	logger.Info().Str("url", "http://localhost:"+port).Msg("console started")
	return cmd, nil
}

// start launches cmd with the run's output and logs it if it exits early.
func start(cmd *exec.Cmd, name string) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stopWithParent(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", name, err)
	}
	go func() {
		if err := cmd.Wait(); err != nil && cmd.ProcessState != nil && !cmd.ProcessState.Success() {
			logger.Warn().Err(err).Str("companion", name).Msg("stopped")
		}
	}()
	return nil
}
