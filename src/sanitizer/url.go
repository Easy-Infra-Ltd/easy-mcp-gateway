package sanitizer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	// urlExtractor matches http/https URLs in text.
	urlExtractor = regexp.MustCompile(`https?://[^\s<>"{}|\\^\x60\[\]]+`)

	// dangerousSchemes matches javascript: and data:text/html URIs.
	dangerousSchemes = regexp.MustCompile(`(?i)(javascript\s*:|data\s*:\s*text/html)`)

	// exfilPatterns matches URL query params that look like data exfiltration.
	exfilPatterns = regexp.MustCompile(`(?i)[?&](secret|token|key|password|api_key|credential|auth|session_id|private_key)=`)
)

// URLScanner detects malicious URLs: dangerous schemes, data exfiltration
// patterns, and suspicious URI types.
type URLScanner struct{}

func (URLScanner) Name() string { return "url" }

func (URLScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	var threats []string

	// Check for dangerous URI schemes anywhere in the content.
	if match := dangerousSchemes.FindString(content); match != "" {
		threats = append(threats, fmt.Sprintf("dangerous URI scheme detected: %q", strings.TrimSpace(match)))
	}

	// Check extracted URLs for exfiltration patterns.
	urls := urlExtractor.FindAllString(content, -1)
	for _, u := range urls {
		if exfilPatterns.MatchString(u) {
			threats = append(threats, fmt.Sprintf("possible data exfiltration URL: %q", u))
		}
	}

	if len(threats) > 0 {
		return ScanResult{
			Verdict:     VerdictBlock,
			Content:     content,
			Threats:     threats,
			ScannerName: "url",
		}, nil
	}

	return ScanResult{
		Verdict:     VerdictPass,
		Content:     content,
		ScannerName: "url",
	}, nil
}
