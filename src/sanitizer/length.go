package sanitizer

import "context"

// LengthScanner truncates content exceeding a character limit.
type LengthScanner struct {
	MaxChars int
}

// NewLengthScanner creates a LengthScanner with the given character limit.
func NewLengthScanner(maxChars int) *LengthScanner {
	return &LengthScanner{MaxChars: maxChars}
}

func (s *LengthScanner) Name() string { return "length" }

func (s *LengthScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	if len([]rune(content)) <= s.MaxChars {
		return ScanResult{
			Verdict:     VerdictPass,
			Content:     content,
			ScannerName: s.Name(),
		}, nil
	}

	runes := []rune(content)
	truncated := string(runes[:s.MaxChars]) + "\n[truncated]"

	return ScanResult{
		Verdict:     VerdictModify,
		Content:     truncated,
		Threats:     []string{"response exceeded character limit"},
		ScannerName: s.Name(),
	}, nil
}
