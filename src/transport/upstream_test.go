package transport

import (
	"context"
	"testing"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewUpstream_createsServer(t *testing.T) {
	u := NewUpstream(config.UpstreamConfig{Transport: config.TransportStdio}, testLogger())
	if u.Server == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestUpstream_runUnsupported(t *testing.T) {
	u := NewUpstream(config.UpstreamConfig{Transport: "grpc"}, testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled
	err := u.Run(ctx)
	if err == nil {
		t.Fatal("expected error for unsupported transport")
	}
}

func TestUpstream_toolRegistration(t *testing.T) {
	u := NewUpstream(config.UpstreamConfig{Transport: config.TransportStdio}, testLogger())

	// Register a tool on the upstream server.
	u.Server.AddTool(&mcp.Tool{
		Name:        "test_tool",
		Description: "a test tool",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "response"}},
		}, nil
	})

	// Connect an in-memory client to verify the tool is discoverable.
	srvTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = u.Server.Run(ctx, srvTransport)
	}()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: "0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	// List tools.
	var tools []*mcp.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("listing tools: %v", err)
		}
		tools = append(tools, tool)
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "test_tool" {
		t.Errorf("expected tool name test_tool, got %s", tools[0].Name)
	}

	// Call the tool.
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "test_tool"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content, got %d", len(result.Content))
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *TextContent, got %T", result.Content[0])
	}
	if tc.Text != "response" {
		t.Errorf("expected text 'response', got %q", tc.Text)
	}
}
