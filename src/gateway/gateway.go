package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/transport"
)

// Gateway is the top-level orchestrator. It wires config, transports,
// tool registry, and the sanitization pipeline together.
type Gateway struct {
	cfg    config.Config
	logger *slog.Logger

	// transportFactory is injected for testing; nil uses the default.
	transportFactory transport.TransportFactory
}

// New creates a Gateway from the given config and logger.
func New(cfg config.Config, logger *slog.Logger) *Gateway {
	return &Gateway{cfg: cfg, logger: logger}
}

// NewWithTransportFactory creates a Gateway with a custom transport factory
// (primarily for testing).
func NewWithTransportFactory(cfg config.Config, logger *slog.Logger, factory transport.TransportFactory) *Gateway {
	return &Gateway{cfg: cfg, logger: logger, transportFactory: factory}
}

// Run starts the gateway: connects downstream, discovers tools, registers
// proxied handlers, and starts the upstream server. Blocks until SIGINT/
// SIGTERM or ctx cancellation.
func (g *Gateway) Run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g.logger.Info("starting gateway")

	// 1. Connect to downstream servers.
	dm, err := transport.NewDownstreamManager(ctx, g.cfg.Downstream, g.logger, g.transportFactory)
	if err != nil {
		return fmt.Errorf("downstream: %w", err)
	}
	defer dm.Close()

	// 2. Create upstream server.
	upstream := transport.NewUpstream(g.cfg.Upstream, g.logger)

	// 3. Discover tools and register proxied handlers.
	reg := NewRegistry(upstream, dm, g.cfg.Sanitization, g.logger)
	count, err := reg.DiscoverAndRegister(ctx)
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	g.logger.Info("tool discovery complete", "total", count)

	// 4. Start upstream (blocks until ctx cancelled).
	g.logger.Info("upstream ready", "transport", g.cfg.Upstream.Transport)
	return upstream.Run(ctx)
}
