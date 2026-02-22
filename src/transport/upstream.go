package transport

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Upstream wraps an MCP server that faces LLM clients. Tools are registered
// on the underlying Server before calling Run.
type Upstream struct {
	Server *mcp.Server
	cfg    config.UpstreamConfig
	logger *slog.Logger
}

// NewUpstream creates an upstream MCP server configured for the given transport.
// Tools should be added to u.Server before calling Run.
func NewUpstream(cfg config.UpstreamConfig, logger *slog.Logger) *Upstream {
	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    "easy-mcp-gateway",
			Version: Version,
		},
		&mcp.ServerOptions{Logger: logger},
	)
	return &Upstream{
		Server: srv,
		cfg:    cfg,
		logger: logger.With("area", "upstream"),
	}
}

// Run starts the upstream server on the configured transport and blocks
// until ctx is cancelled or the transport closes.
func (u *Upstream) Run(ctx context.Context) error {
	switch u.cfg.Transport {
	case config.TransportStdio:
		return u.runStdio(ctx)
	case config.TransportHTTP:
		return u.runHTTP(ctx)
	default:
		return fmt.Errorf("unsupported upstream transport: %s", u.cfg.Transport)
	}
}

func (u *Upstream) runStdio(ctx context.Context) error {
	u.logger.Info("starting stdio transport")
	return u.Server.Run(ctx, &mcp.StdioTransport{})
}

func (u *Upstream) runHTTP(ctx context.Context) error {
	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return u.Server },
		&mcp.StreamableHTTPOptions{Logger: u.logger},
	)

	mux := http.NewServeMux()
	mux.Handle(u.cfg.HTTP.Path, handler)

	ln, err := net.Listen("tcp", u.cfg.HTTP.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", u.cfg.HTTP.Addr, err)
	}
	u.logger.Info("starting HTTP transport", "addr", ln.Addr(), "path", u.cfg.HTTP.Path)

	srv := &http.Server{Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		u.logger.Info("shutting down HTTP transport")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
