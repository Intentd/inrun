//go:build !runtime

package internal

import (
	"context"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/app"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/utils"
)

// RunGateway builds and runs the gateway until ctx ends. There is no leader
// election: the gateway is stateless and runs as several replicas.
func RunGateway(ctx context.Context, cfg *config.Config, m *merger.Merger, opts app.GatewayOptions) {
	gw, err := app.New(ctx, cfg, m, opts)
	if err != nil {
		utils.FatalExit("gateway startup failed", err)
	}

	if gw == nil {
		utils.FatalExit("app.New returned no gateway", nil)
	}

	go func() {
		if err := gw.Process.Start(ctx); err != nil {
			utils.FatalExit("gateway startup error", err)
		}
	}()

	logger.Info().Msg("gateway started")

	gw.Process.Wait()
}
