// Package gateway wires upstream and downstream transports together,
// proxying tool calls through a sanitization pipeline.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/config"
	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/sanitizer"
	"github.com/Easy-Infra-Ltd/easy-mcp-gateway/src/transport"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const namespaceSep = "__"

// Registry discovers tools from downstream servers, namespaces them, and
// registers proxy handlers on the upstream server. Each proxy call runs
// responses through the sanitization pipeline.
type Registry struct {
	upstream   *transport.Upstream
	downstream *transport.DownstreamManager
	globalCfg  config.SanitizationConfig
	logger     *slog.Logger
}

// NewRegistry creates a registry wired to the given upstream/downstream pair.
func NewRegistry(
	upstream *transport.Upstream,
	downstream *transport.DownstreamManager,
	globalCfg config.SanitizationConfig,
	logger *slog.Logger,
) *Registry {
	return &Registry{
		upstream:   upstream,
		downstream: downstream,
		globalCfg:  globalCfg,
		logger:     logger.With("area", "registry"),
	}
}

// DiscoverAndRegister iterates all downstream connections, discovers their
// tools, and registers namespaced proxy handlers on the upstream server.
// Returns the total number of tools registered.
func (r *Registry) DiscoverAndRegister(ctx context.Context) (int, error) {
	total := 0

	for name, conn := range r.downstream.Conns() {
		merged := config.Merge(&r.globalCfg, conn.Config.Sanitization)

		pipeline, err := BuildPipeline(merged, name)
		if err != nil {
			return total, fmt.Errorf("building pipeline for %s: %w", name, err)
		}

		count, err := r.registerServer(ctx, name, conn.Session, pipeline)
		if err != nil {
			return total, fmt.Errorf("registering tools for %s: %w", name, err)
		}

		r.logger.Info("registered tools", "server", name, "count", count)
		total += count
	}

	if total == 0 {
		return 0, fmt.Errorf("no tools discovered from any downstream server")
	}
	return total, nil
}

func (r *Registry) registerServer(
	ctx context.Context,
	serverName string,
	session *mcp.ClientSession,
	pipeline *sanitizer.Pipeline,
) (int, error) {
	count := 0
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return count, fmt.Errorf("listing tools: %w", err)
		}

		namespacedName := serverName + namespaceSep + tool.Name

		proxied := proxyTool(tool, namespacedName)
		handler := proxyHandler(r.downstream, serverName, tool.Name, namespacedName, pipeline, r.logger)
		r.upstream.Server.AddTool(proxied, handler)

		count++
	}
	return count, nil
}

// proxyTool creates a copy of the downstream tool with a namespaced name.
func proxyTool(original *mcp.Tool, namespacedName string) *mcp.Tool {
	return &mcp.Tool{
		Name:        namespacedName,
		Description: original.Description,
		InputSchema: original.InputSchema,
		Annotations: original.Annotations,
		Title:       original.Title,
	}
}

// proxyHandler returns a ToolHandler that forwards calls to the downstream
// session, then sanitizes the response. It looks up the session at call time
// so that reconnected sessions are used automatically.
func proxyHandler(
	dm *transport.DownstreamManager,
	serverName string,
	downstreamName string,
	namespacedName string,
	pipeline *sanitizer.Pipeline,
	logger *slog.Logger,
) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		session := dm.Session(serverName)
		if session == nil {
			return nil, fmt.Errorf("downstream %s not connected", serverName)
		}

		// Forward to downstream with original tool name.
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      downstreamName,
			Arguments: req.Params.Arguments,
		})
		if err != nil {
			return nil, fmt.Errorf("downstream call %s: %w", namespacedName, err)
		}

		// Sanitize each text content item.
		return sanitizeResult(ctx, result, pipeline, logger)
	}
}

// sanitizeResult runs each TextContent through the pipeline.
// On Block: replaces entire result with an IsError response.
// On Modify: replaces text content with sanitized version.
func sanitizeResult(
	ctx context.Context,
	result *mcp.CallToolResult,
	pipeline *sanitizer.Pipeline,
	logger *slog.Logger,
) (*mcp.CallToolResult, error) {
	for i, content := range result.Content {
		tc, ok := content.(*mcp.TextContent)
		if !ok {
			continue
		}

		pr, err := pipeline.Process(ctx, tc.Text)
		if err != nil {
			return nil, err
		}

		switch pr.FinalVerdict {
		case sanitizer.VerdictBlock:
			reason := "blocked by sanitization"
			if len(pr.AllThreats) > 0 {
				reason = strings.Join(pr.AllThreats, "; ")
			}
			logger.Warn("blocked tool response",
				"threats", pr.AllThreats,
			)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: reason}},
				IsError: true,
			}, nil

		case sanitizer.VerdictModify:
			result.Content[i] = &mcp.TextContent{
				Text:        pr.FinalContent,
				Annotations: tc.Annotations,
			}
		}
	}

	return result, nil
}

// BuildPipeline constructs a sanitizer.Pipeline from a (merged) config.
// Scanner order: unicode -> length -> injection -> override -> url -> boundary.
func BuildPipeline(cfg config.SanitizationConfig, source string) (*sanitizer.Pipeline, error) {
	var scanners []sanitizer.Scanner

	if deref(cfg.EnableInvisibleTextRemoval) {
		scanners = append(scanners, &sanitizer.UnicodeScanner{})
	}

	if cfg.MaxResponseChars != nil && *cfg.MaxResponseChars > 0 {
		scanners = append(scanners, sanitizer.NewLengthScanner(*cfg.MaxResponseChars))
	}

	if deref(cfg.EnablePromptInjectionDetection) {
		s, err := sanitizer.NewInjectionScanner(
			deref(cfg.DisableBuiltInPatterns),
			cfg.CustomInjectionPatterns,
		)
		if err != nil {
			return nil, fmt.Errorf("injection scanner: %w", err)
		}
		scanners = append(scanners, s)
	}

	if deref(cfg.EnableSystemOverrideDetection) {
		scanners = append(scanners, &sanitizer.OverrideScanner{})
	}

	if deref(cfg.EnableURLValidation) {
		scanners = append(scanners, &sanitizer.URLScanner{})
	}

	if deref(cfg.EnableBoundaryInjection) {
		scanners = append(scanners, sanitizer.NewBoundaryScanner(source))
	}

	return sanitizer.NewPipeline(scanners...), nil
}

func deref(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
