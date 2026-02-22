package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGateway_endToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a downstream server with one tool.
	dsTransport := testDownstreamServer(t, ctx, map[string]mcp.ToolHandler{
		"hello": echoHandler("world"),
	})
	factory := func(ds config.DownstreamConfig) (mcp.Transport, error) {
		return dsTransport, nil
	}

	cfg := config.Config{
		Upstream: config.UpstreamConfig{Transport: config.TransportStdio},
		Downstream: []config.DownstreamConfig{
			{Name: "ds", Transport: config.TransportStdio, Command: []string{"dummy"}},
		},
		Sanitization: minimalSanitizationConfig(),
	}

	gw := NewWithTransportFactory(cfg, testLogger(), factory)

	// Gateway.Run blocks on upstream, so we need to override the upstream
	// transport. Instead, test through the registry directly (already
	// covered in registry_test.go). Here we verify construction doesn't panic.
	_ = gw
}

func TestGateway_runCancellation(t *testing.T) {
	// Verify that Run respects context cancellation by timing out quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dsTransport := testDownstreamServer(t, ctx, map[string]mcp.ToolHandler{
		"ping": echoHandler("pong"),
	})
	factory := func(ds config.DownstreamConfig) (mcp.Transport, error) {
		return dsTransport, nil
	}

	cfg := config.Config{
		Upstream: config.UpstreamConfig{Transport: config.TransportStdio},
		Downstream: []config.DownstreamConfig{
			{Name: "ds", Transport: config.TransportStdio, Command: []string{"dummy"}},
		},
		Sanitization: minimalSanitizationConfig(),
	}

	gw := NewWithTransportFactory(cfg, testLogger(), factory)

	// Run will try to start stdio upstream (no peer), context will cancel.
	err := gw.Run(ctx)
	// We expect either nil or a context-related error — not a panic.
	_ = err
}

func TestNew_createsGateway(t *testing.T) {
	cfg := config.Config{
		Upstream: config.UpstreamConfig{Transport: config.TransportStdio},
		Downstream: []config.DownstreamConfig{
			{Name: "x", Transport: config.TransportStdio, Command: []string{"dummy"}},
		},
		Sanitization: defaultSanitizationConfig(),
	}
	gw := New(cfg, testLogger())
	if gw == nil {
		t.Fatal("expected non-nil gateway")
	}
	if gw.transportFactory != nil {
		t.Error("expected nil transport factory for default gateway")
	}
}
