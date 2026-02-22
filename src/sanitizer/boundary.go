package sanitizer

import (
	"context"
	"fmt"
)

// BoundaryScanner wraps content in XML-style delimiters to help the LLM
// distinguish external tool output from its own instructions.
type BoundaryScanner struct {
	Source string // e.g. "servername__toolname"
}

// NewBoundaryScanner creates a BoundaryScanner for the given source label.
func NewBoundaryScanner(source string) *BoundaryScanner {
	return &BoundaryScanner{Source: source}
}

func (s *BoundaryScanner) Name() string { return "boundary" }

func (s *BoundaryScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	wrapped := fmt.Sprintf("<external_tool_response source=%q>\n%s\n</external_tool_response>", s.Source, content)

	return ScanResult{
		Verdict:     VerdictModify,
		Content:     wrapped,
		ScannerName: s.Name(),
	}, nil
}
