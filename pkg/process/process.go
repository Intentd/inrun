package process

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
)

const eventHandler = "event handler"

type Manager struct {
	components      []domain.Component
	postStart       []postStart
	shutdownHooks   []func(context.Context) // called after all components stop
	timeout         time.Duration
	done            chan struct{}
	runningInstance string // Which instance (runtime or gateway) is running
}

type postStart struct {
	hook func(context.Context)
	comp domain.Component
}

// New returns the Manager for this process's instance (runtime or gateway)
// with components registered in start order. They stop in reverse order
// within the Blueprint's shutdown grace period.
func New(cfg *config.Config, components []domain.Component) *Manager {
	m := &Manager{
		runningInstance: cfg.RunningInstance(),
		timeout:         cfg.Blueprint().ShutdownGracePeriod(),
		done:            make(chan struct{}),
	}
	m.Register(components)
	return m
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
func (m *Manager) OnShutdown(fn func(context.Context)) {
	m.shutdownHooks = append(m.shutdownHooks, fn)
}

func (m *Manager) Start(ctx context.Context) error {
	mCtx, mCancel := context.WithCancel(ctx)
	defer mCancel()

	for _, comp := range m.components {
		name := comp.Name()

		logger.Debug().Msgf("[%s] starting...", name)
		if err := comp.Start(mCtx); err != nil {
			logger.Error().Err(err).Msgf("failed to start: %s", name)
			return err
		}
		logger.Debug().Msgf("%s status: %v", name, "online")
	}

	// Run post-start hooks (leader election goes here)
	for _, p := range m.postStart {
		go p.hook(mCtx)
	}

	names := m.componentNames()
	for _, p := range m.postStart {
		names = append(names, p.comp.Name())
	}
	logger.Info().Str("instance", m.runningInstance).Strs("components", names).Msg("running")

	m.gracefulShutdown(mCtx, mCancel)
	return nil
}

func (m *Manager) Shutdown(ctx context.Context) {}

func (m *Manager) gracefulShutdown(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Warn().Msgf("received shutdown signal: %v", sig)
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(
			context.Background(), m.timeout,
		)
		defer shutdownCancel()

		// Stop components in reverse start order
		for _, comp := range structured.Reversed(m.components) {
			// Respect the timeout between iterations
			select {
			case <-shutdownCtx.Done():
				logger.Warn().Msg("shutdown timeout exceeded — stopping")
				close(m.done)
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
		if ev := m.GetComponent(eventHandler); ev != nil {
			ev.Shutdown(shutdownCtx)
			logger.Warn().Msgf("%s: offline", ev.Name())
		}

		// Run shutdown hooks after all components have stopped
		// Hooks run in registration order — RBAC cleanup, webhook removal, etc.
		for i, hook := range m.shutdownHooks {
			select {
			case <-shutdownCtx.Done():
				logger.Warn().
					Int("remaining", len(m.shutdownHooks)-i).
					Msg("shutdown timeout exceeded — skipping remaining hooks")
				close(m.done)
				return
			default:
			}
			hook(shutdownCtx)
		}

		logger.Warn().Msg("all components shut down gracefully")
		close(m.done)

	case <-ctx.Done():
		return
	}
}

// Register adds components in start order.
func (m *Manager) Register(c []domain.Component) {
	m.components = append(m.components, c...)
	logger.Info().Str("instance", m.runningInstance).Strs("components", m.componentNames()).Msg("registered")
}

// componentNames lists the registered components in start order.
func (m *Manager) componentNames() []string {
	names := make([]string, 0, len(m.components))
	for _, comp := range m.components {
		names = append(names, comp.Name())
	}
	return names
}

// GetComponent returns a component if present
func (m *Manager) GetComponent(name string) domain.Component {
	for _, comp := range m.components {
		if comp.Name() == name {
			return comp
		}
	}
	return nil
}

// AddPostStartHook: for services that need to start after the manager has started
func (m *Manager) AddPostStartHook(comp domain.Component, hook func(context.Context)) {
	m.postStart = append(m.postStart, postStart{
		hook: hook,
		comp: comp,
	})
}

// Listening to done channel
func (m *Manager) Wait() {
	<-m.done
}
