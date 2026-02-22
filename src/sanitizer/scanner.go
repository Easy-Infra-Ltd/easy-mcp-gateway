// Package sanitizer provides a pipeline of content scanners that detect
// and mitigate prompt injection, malicious content, and other threats in
// MCP tool responses before they reach the LLM.
package sanitizer

import "context"

// Scanner inspects and optionally transforms text content.
// Implementations must not mutate the input; return transformed
// content in the ScanResult.
type Scanner interface {
	// Name returns a human-readable identifier for logging/metrics.
	Name() string

	// Scan inspects content and returns a ScanResult.
	Scan(ctx context.Context, content string) (ScanResult, error)
}
