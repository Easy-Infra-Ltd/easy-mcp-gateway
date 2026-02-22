package sanitizer

import (
	"context"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// UnicodeScanner removes invisible and potentially malicious Unicode
// characters and normalizes text to NFKC form.
type UnicodeScanner struct{}

func (UnicodeScanner) Name() string { return "unicode" }

func (UnicodeScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	normalized := norm.NFKC.String(content)

	var b strings.Builder
	b.Grow(len(normalized))

	removed := 0
	for _, r := range normalized {
		if shouldRemove(r) {
			removed++
			continue
		}
		b.WriteRune(r)
	}

	cleaned := b.String()

	if removed == 0 && cleaned == content {
		return ScanResult{
			Verdict:     VerdictPass,
			Content:     content,
			ScannerName: "unicode",
		}, nil
	}

	threats := make([]string, 0, 1)
	if removed > 0 {
		threats = append(threats, "invisible/control characters removed")
	}

	return ScanResult{
		Verdict:     VerdictModify,
		Content:     cleaned,
		Threats:     threats,
		ScannerName: "unicode",
	}, nil
}

// shouldRemove returns true for characters that should be stripped.
// Removes Unicode categories Cf (format), Co (private use), Cn (unassigned),
// and Cc (control) — except for common whitespace characters.
func shouldRemove(r rune) bool {
	// Keep common whitespace.
	if r == '\n' || r == '\t' || r == '\r' || r == ' ' {
		return false
	}

	cat := unicode.In(r,
		unicode.Cf, // Format (zero-width joiners, directional marks, etc.)
		unicode.Co, // Private use
		unicode.Cc, // Control
	)
	return cat
}
