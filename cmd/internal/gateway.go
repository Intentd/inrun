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
func RunGateway(cfg *config.Config, m *merger.Merger, ctx context.Context) {
	gw := app.New(cfg, m, ctx)

	go func() {
		if err := gw.Process.Start(ctx); err != nil {
			logger.Fatal().AnErr("gateway startup error", err)
			utils.Exit(err)
		}
	}()

	logger.Info().Msg("gateway started")

	gw.Process.Wait()
}
