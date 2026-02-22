package transport

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// testServer starts an in-memory MCP server and returns the client-side
// transport. The server runs until ctx is cancelled.
func testServer(t *testing.T, ctx context.Context) mcp.Transport {
	t.Helper()
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "test-server", Version: "0.0.1"},
		nil,
	)
	// Add a dummy tool so the server advertises tools capability.
	srv.AddTool(&mcp.Tool{
		Name:        "echo",
		Description: "echoes input",
		InputSchema: map[string]any{"type": "object"},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	})

	srvTransport, clientTransport := mcp.NewInMemoryTransports()
	go func() {
		_ = srv.Run(ctx, srvTransport)
	}()

	return clientTransport
}

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestNewDownstreamManager_connects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientTransport := testServer(t, ctx)
	factory := singleTransportFactory(clientTransport)

	dm, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "srv1", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), factory)
	if err != nil {
		t.Fatalf("NewDownstreamManager: %v", err)
	}
	defer dm.Close()

	if s := dm.Session("srv1"); s == nil {
		t.Fatal("expected session for srv1, got nil")
	}
}

func TestNewDownstreamManager_multipleServers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Each downstream needs its own in-memory server pair.
	transports := map[string]mcp.Transport{
		"a": testServer(t, ctx),
		"b": testServer(t, ctx),
	}
	factory := namedTransportFactory(transports)

	dm, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "a", Transport: config.TransportStdio, Command: []string{"dummy"}},
		{Name: "b", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), factory)
	if err != nil {
		t.Fatalf("NewDownstreamManager: %v", err)
	}
	defer dm.Close()

	conns := dm.Conns()
	if len(conns) != 2 {
		t.Fatalf("expected 2 conns, got %d", len(conns))
	}
	for _, name := range []string{"a", "b"} {
		if conns[name] == nil {
			t.Errorf("missing conn for %s", name)
		}
	}
}

func TestNewDownstreamManager_allFail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	factory := func(_ config.DownstreamConfig) (mcp.Transport, error) {
		return nil, errTestConnect
	}

	_, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "bad", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), factory)
	if err == nil {
		t.Fatal("expected error when all connections fail")
	}
}

func TestNewDownstreamManager_partialFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	goodTransport := testServer(t, ctx)

	callCount := 0
	factory := func(ds config.DownstreamConfig) (mcp.Transport, error) {
		callCount++
		if ds.Name == "bad" {
			return nil, errTestConnect
		}
		return goodTransport, nil
	}

	dm, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "good", Transport: config.TransportStdio, Command: []string{"dummy"}},
		{Name: "bad", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), factory)
	if err != nil {
		t.Fatalf("should succeed with partial connections: %v", err)
	}
	defer dm.Close()

	if dm.Session("good") == nil {
		t.Error("expected session for good")
	}
	if dm.Session("bad") != nil {
		t.Error("expected nil session for bad")
	}
}

func TestSession_unknownName(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dm, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "s", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), singleTransportFactory(testServer(t, ctx)))
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	if s := dm.Session("nonexistent"); s != nil {
		t.Error("expected nil for unknown name")
	}
}

func TestClose_clearsConns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dm, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "s", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), singleTransportFactory(testServer(t, ctx)))
	if err != nil {
		t.Fatal(err)
	}

	dm.Close()
	if len(dm.Conns()) != 0 {
		t.Error("expected empty conns after Close")
	}
}

func TestNewTransport_stdio(t *testing.T) {
	ds := config.DownstreamConfig{
		Transport: config.TransportStdio,
		Command:   []string{"echo", "hello"},
	}
	tr, err := newTransport(ds)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	if _, ok := tr.(*mcp.CommandTransport); !ok {
		t.Errorf("expected *mcp.CommandTransport, got %T", tr)
	}
}

func TestNewTransport_http(t *testing.T) {
	ds := config.DownstreamConfig{
		Transport: config.TransportHTTP,
		URL:       "http://localhost:9999/mcp",
	}
	tr, err := newTransport(ds)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	if _, ok := tr.(*mcp.StreamableClientTransport); !ok {
		t.Errorf("expected *mcp.StreamableClientTransport, got %T", tr)
	}
}

func TestNewTransport_stdioMissingCommand(t *testing.T) {
	_, err := newTransport(config.DownstreamConfig{Transport: config.TransportStdio})
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestNewTransport_httpMissingURL(t *testing.T) {
	_, err := newTransport(config.DownstreamConfig{Transport: config.TransportHTTP})
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestNewTransport_unsupported(t *testing.T) {
	_, err := newTransport(config.DownstreamConfig{Transport: "grpc"})
	if err == nil {
		t.Error("expected error for unsupported transport")
	}
}

func TestHealthCheck_reconnects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start with a working server.
	goodTransport := testServer(t, ctx)

	var mu sync.Mutex
	reconnected := false

	factory := func(ds config.DownstreamConfig) (mcp.Transport, error) {
		mu.Lock()
		defer mu.Unlock()
		if reconnected {
			// Second call (reconnection): provide a fresh server.
			return testServer(t, ctx), nil
		}
		return goodTransport, nil
	}

	dm, err := NewDownstreamManager(ctx, []config.DownstreamConfig{
		{Name: "s", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}, testLogger(), factory)
	if err != nil {
		t.Fatal(err)
	}
	defer dm.Close()

	// Close the original session to simulate failure, then trigger reconnection.
	dm.mu.Lock()
	conn := dm.conns["s"]
	dm.mu.Unlock()
	_ = conn.Session.Close()

	mu.Lock()
	reconnected = true
	mu.Unlock()

	// Manually trigger checkAndReconnect.
	cfgs := map[string]config.DownstreamConfig{
		"s": {Name: "s", Transport: config.TransportStdio, Command: []string{"dummy"}},
	}
	dm.checkAndReconnect(ctx, cfgs)

	// Allow reconnection to complete.
	time.Sleep(50 * time.Millisecond)

	newSession := dm.Session("s")
	if newSession == nil {
		t.Fatal("expected reconnected session")
	}
	if newSession == conn.Session {
		t.Error("expected a different session after reconnection")
	}
}

// --- helpers ---

var errTestConnect = fmt.Errorf("test connect error")

// singleTransportFactory returns a factory that always provides the same transport.
// Only suitable when a single downstream is configured.
func singleTransportFactory(t mcp.Transport) TransportFactory {
	return func(_ config.DownstreamConfig) (mcp.Transport, error) {
		return t, nil
	}
}

// namedTransportFactory returns a factory that maps downstream names to transports.
func namedTransportFactory(m map[string]mcp.Transport) TransportFactory {
	return func(ds config.DownstreamConfig) (mcp.Transport, error) {
		t, ok := m[ds.Name]
		if !ok {
			return nil, fmt.Errorf("no transport for %s", ds.Name)
		}
		return t, nil
	}
}
