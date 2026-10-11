package process

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/utils"
)

const eventHandler = "event handler"

type Manager struct {
	components      []domain.Component
	postStart       []postStart
	shutdownHooks   []func(context.Context) // called after all components stop
	timeout         time.Duration
	logLevel        string
	done            chan struct{}
	runningInstance string // Which instance (runtime or gateway) is running
}

type postStart struct {
	hook func(context.Context)
	comp domain.Component
}

func New(instance string, timeout time.Duration, logLevel string) *Manager {
	return &Manager{
		runningInstance: instance,
		timeout:         timeout,
		logLevel:        logLevel,
		done:            make(chan struct{}),
	}
}

// OnShutdown registers a function to be called after all components have
// stopped, within the graceful shutdown timeout.
//
// Use this for cleanup that must happen after the operator stops processing
// but before the process exits:
//   - RBAC deletion (security.rbac.cleanupOnShutdown: true)
//   - Deletion protection webhook removal
//   - Temp file cleanup (generated TLS certs)
//
// Hooks are called in registration order, sequentially.
// If the shutdown timeout is exceeded before all hooks run, remaining hooks
// are skipped — the process is exiting regardless.
func (o *Manager) OnShutdown(fn func(context.Context)) {
	o.shutdownHooks = append(o.shutdownHooks, fn)
}

func (o *Manager) Start(ctx context.Context) error {
	mCtx, mCancel := context.WithCancel(ctx)
	defer mCancel()

	logger.Info().Msgf("Starting %s components...", o.runningInstance)
	for _, comp := range o.components {
		name := comp.Name()

		logger.Debug().Msgf("[%s] starting...", name)
		if err := comp.Start(mCtx); err != nil {
			logger.Error().Err(err).Msgf("failed to start: %s", name)
			return err
		}
		utils.Sleep(1)
		logger.Debug().Msgf("%s status: %v", name, utils.StatusOnline)
	}

	// Run post-start hooks (leader election goes here)
	logger.Info().Msg("Running post-start hooks...")
	for _, p := range o.postStart {
		go p.hook(mCtx)
	}

	logger.Info().Msgf("%s All components started successfully", utils.SuccessMarkPlain())

	if strings.ToLower(o.logLevel) == "debug" {
		// Display started components
		fmt.Println("===============================")
		fmt.Println("STARTED COMPONENTS:")

		n := 1
		for _, comp := range o.components {
			fmt.Printf("%d. %s\n", n, comp.Name())
			n++
		}

		for _, p := range o.postStart {
			fmt.Printf("%d. %s\n", n, p.comp.Name())
			n++
		}
		fmt.Println("===============================")

	}

	logger.Info().Msgf("%s Inrun %s is running...", utils.SuccessMarkPlain(), o.runningInstance)

	o.gracefulShutdown(mCtx, mCancel)
	return nil
}

func (o *Manager) Shutdown(ctx context.Context) {}

func (o *Manager) gracefulShutdown(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Warn().Msgf("received shutdown signal: %v", sig)
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(), o.timeout,
		)
		defer shutdownCancel()

		// Stop components in reverse start order
		for _, comp := range utils.Reversed(o.components) {
			// Respect the timeout between iterations
			select {
			case <-shutdownCtx.Done():
				logger.Warn().Msg("shutdown timeout exceeded — stopping")
				close(o.done)
				return
			default:
			}

			name := comp.Name()
			if name == eventHandler {
				continue
			}

			logger.Info().Msgf("shutting down: %s...", name)
			comp.Shutdown(shutdownCtx)
			logger.Warn().Msgf("%s: offline", name)
		}

		// Event handler always last
		if ev := o.GetComponent(eventHandler); ev != nil {
			ev.Shutdown(shutdownCtx)
			logger.Warn().Msgf("%s: offline", ev.Name())
		}

		// Run shutdown hooks after all components have stopped
		// Hooks run in registration order — RBAC cleanup, webhook removal, etc.
		for i, hook := range o.shutdownHooks {
			select {
			case <-shutdownCtx.Done():
				logger.Warn().
					Int("remaining", len(o.shutdownHooks)-i).
					Msg("shutdown timeout exceeded — skipping remaining hooks")
				close(o.done)
				return
			default:
			}
			hook(shutdownCtx)
		}

		logger.Warn().Msg("all components shut down gracefully")
		close(o.done)

	case <-ctx.Done():
		return
	}
}

// Register all components
func (o *Manager) Register(c []domain.Component) {
	logger.Info().Msgf("Registering inrun %s components...", o.runningInstance)
	for _, comp := range c {
		o.components = append(o.components, comp)
		logger.Info().Msgf("[%s] registered", comp.Name())
	}
	logger.Info().Msgf("%s All components registered successfully", utils.SuccessMarkPlain())

	if strings.ToLower(o.logLevel) == "debug" {
		// Display registered components
		fmt.Println("==================================")
		fmt.Println("REGISTERED COMPONENTS:")
		n := 1
		for _, comp := range o.components {
			fmt.Printf("%d. %s\n", n, comp.Name())
			n++
		}
	}
}

// GetComponent returns a component if present
func (o *Manager) GetComponent(name string) domain.Component {
	for _, comp := range o.components {
		if comp.Name() == name {
			return comp
		}
	}
	return nil
}

// AddPostStartHook: for services that need to start after the manager has started
func (o *Manager) AddPostStartHook(comp domain.Component, hook func(context.Context)) {
	o.postStart = append(o.postStart, postStart{
		hook: hook,
		comp: comp,
	})
}

// Listening to done channel
func (o *Manager) Wait() {
	<-o.done
}
