package sanitizer

import (
	"context"
	"fmt"
	"regexp"
)

// overridePatterns detect attempts to reassign the LLM's role or persona.
var overridePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)you\s+are\s+(now\s+)?acting\s+as`),
	regexp.MustCompile(`(?i)(roleplay|role-play|role\s+play)\s+as`),
	regexp.MustCompile(`(?i)your\s+(new\s+)?(role|persona|identity)\s+(is|:)`),
	regexp.MustCompile(`(?i)pretend\s+(to\s+be|you\s+are)`),
	regexp.MustCompile(`(?i)system\s*:\s*you\s+are`),
	regexp.MustCompile(`(?i)switch\s+to\s+.*(mode|persona|role)`),
}

// OverrideScanner detects attempts to override the system prompt or
// reassign the LLM's identity/role.
type OverrideScanner struct{}

func (OverrideScanner) Name() string { return "override" }

func (OverrideScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	for _, re := range overridePatterns {
		if match := re.FindString(content); match != "" {
			return ScanResult{
				Verdict:     VerdictBlock,
				Content:     content,
				Threats:     []string{fmt.Sprintf("system prompt override detected: matched %q", re.String())},
				ScannerName: "override",
			}, nil
		}
	}

	return ScanResult{
		Verdict:     VerdictPass,
		Content:     content,
		ScannerName: "override",
	}, nil
}
