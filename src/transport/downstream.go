// Package transport manages MCP transport connections for upstream
// (client-facing) and downstream (server-facing) communication.
package transport

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DownstreamConn holds a live client session to a downstream MCP server
// along with the config that created it.
type DownstreamConn struct {
	Name    string
	Session *mcp.ClientSession
	Config  config.DownstreamConfig
}

// TransportFactory creates a Transport for a given downstream config.
// Exists to allow injection of test transports.
type TransportFactory func(config.DownstreamConfig) (mcp.Transport, error)

// DownstreamManager manages persistent connections to downstream MCP servers
// with health checking and reconnection.
type DownstreamManager struct {
	mu               sync.RWMutex
	conns            map[string]*DownstreamConn
	logger           *slog.Logger
	transportFactory TransportFactory

	// cancelHealthCheck stops the background health check goroutine.
	cancelHealthCheck context.CancelFunc
}

// NewDownstreamManager creates a manager and connects to all configured
// downstream servers. Connections that fail are logged but do not prevent
// startup — they will be retried by health checks.
//
// If transportFactory is nil, the default factory (stdio/HTTP) is used.
func NewDownstreamManager(ctx context.Context, downstream []config.DownstreamConfig, logger *slog.Logger, transportFactory TransportFactory) (*DownstreamManager, error) {
	if transportFactory == nil {
		transportFactory = newTransport
	}
	dm := &DownstreamManager{
		conns:            make(map[string]*DownstreamConn, len(downstream)),
		logger:           logger.With("area", "downstream"),
		transportFactory: transportFactory,
	}

	for _, ds := range downstream {
		conn, err := dm.connect(ctx, ds)
		if err != nil {
			dm.logger.Error("failed to connect", "server", ds.Name, "err", err)
			continue
		}
		dm.conns[ds.Name] = conn
		dm.logger.Info("connected", "server", ds.Name, "transport", ds.Transport)
	}

	if len(dm.conns) == 0 {
		return nil, fmt.Errorf("failed to connect to any downstream servers")
	}

	hctx, cancel := context.WithCancel(ctx)
	dm.cancelHealthCheck = cancel
	go dm.healthCheckLoop(hctx, downstream)

	return dm, nil
}

// Session returns the active session for a named downstream server.
// Returns nil if the server is not connected.
func (dm *DownstreamManager) Session(name string) *mcp.ClientSession {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	conn, ok := dm.conns[name]
	if !ok {
		return nil
	}
	return conn.Session
}

// Conns returns a snapshot of all active connections.
func (dm *DownstreamManager) Conns() map[string]*DownstreamConn {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	out := make(map[string]*DownstreamConn, len(dm.conns))
	for k, v := range dm.conns {
		out[k] = v
	}
	return out
}

// Close terminates all downstream connections and stops health checks.
func (dm *DownstreamManager) Close() {
	if dm.cancelHealthCheck != nil {
		dm.cancelHealthCheck()
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()
	for name, conn := range dm.conns {
		if err := conn.Session.Close(); err != nil {
			dm.logger.Error("error closing session", "server", name, "err", err)
		}
	}
	dm.conns = make(map[string]*DownstreamConn)
}

func (dm *DownstreamManager) connect(ctx context.Context, ds config.DownstreamConfig) (*DownstreamConn, error) {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "easy-mcp-gateway",
			Version: "0.1.0",
		},
		&mcp.ClientOptions{Logger: dm.logger},
	)

	transport, err := dm.transportFactory(ds)
	if err != nil {
		return nil, fmt.Errorf("creating transport for %s: %w", ds.Name, err)
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", ds.Name, err)
	}

	return &DownstreamConn{
		Name:    ds.Name,
		Session: session,
		Config:  ds,
	}, nil
}

func newTransport(ds config.DownstreamConfig) (mcp.Transport, error) {
	switch ds.Transport {
	case config.TransportStdio:
		if len(ds.Command) == 0 {
			return nil, fmt.Errorf("stdio transport requires a command")
		}
		cmd := exec.Command(ds.Command[0], ds.Command[1:]...)
		return &mcp.CommandTransport{Command: cmd}, nil

	case config.TransportHTTP:
		if ds.URL == "" {
			return nil, fmt.Errorf("http transport requires a url")
		}
		return &mcp.StreamableClientTransport{Endpoint: ds.URL}, nil

	default:
		return nil, fmt.Errorf("unsupported transport: %s", ds.Transport)
	}
}

const healthCheckInterval = 30 * time.Second

func (dm *DownstreamManager) healthCheckLoop(ctx context.Context, downstream []config.DownstreamConfig) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	// Index configs by name for reconnection.
	cfgByName := make(map[string]config.DownstreamConfig, len(downstream))
	for _, ds := range downstream {
		cfgByName[ds.Name] = ds
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dm.checkAndReconnect(ctx, cfgByName)
		}
	}
}

func (dm *DownstreamManager) checkAndReconnect(ctx context.Context, cfgs map[string]config.DownstreamConfig) {
	if ctx.Err() != nil {
		return
	}

	for name, cfg := range cfgs {
		dm.mu.RLock()
		conn, connected := dm.conns[name]
		dm.mu.RUnlock()

		if connected {
			// Ping to verify liveness.
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Session.Ping(pingCtx, &mcp.PingParams{})
			cancel()
			if err == nil {
				continue
			}
			dm.logger.Warn("health check failed, reconnecting", "server", name, "err", err)
			_ = conn.Session.Close()
		}

		// Attempt reconnection.
		newConn, err := dm.connect(ctx, cfg)
		if err != nil {
			dm.logger.Error("reconnect failed", "server", name, "err", err)
			dm.mu.Lock()
			delete(dm.conns, name)
			dm.mu.Unlock()
			continue
		}

		dm.mu.Lock()
		dm.conns[name] = newConn
		dm.mu.Unlock()
		dm.logger.Info("reconnected", "server", name)
	}
}
