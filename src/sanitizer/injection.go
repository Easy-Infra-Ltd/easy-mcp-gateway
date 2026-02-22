package sanitizer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// builtInInjectionPatterns are regex patterns matching common prompt
// injection phrases. All are compiled with case-insensitive flag.
var builtInInjectionPatterns = []string{
	`ignore\s+(all\s+)?(previous|prior|above)\s+(instructions?|prompts?|context)`,
	`disregard\s+(all\s+)?(previous|prior|above)`,
	`forget\s+(everything|all|your)\s+(instructions?|rules|guidelines|training)`,
	`forget\s+everything`,
	`you\s+are\s+now\s+(a|an|the)\s+`,
	`new\s+instructions?\s*:`,
	`from\s+now\s+on,?\s+you\s+(are|will|must|should)`,
	`<\|?im_start\|?>`,
	`<\|?system\|?>`,
	`###\s*(System|Instructions?|Rules)\s*\n`,
	`\[INST\]`,
	`\[/INST\]`,
	`<<SYS>>`,
	`<</SYS>>`,
	`IMPORTANT:\s*ignore`,
	`CRITICAL:\s*override`,
}

// InjectionScanner detects prompt injection patterns via regex matching.
type InjectionScanner struct {
	patterns []*regexp.Regexp
}

// NewInjectionScanner builds a scanner from the given configuration.
// If disableBuiltIn is false, built-in patterns are included.
// customPatterns are always appended.
func NewInjectionScanner(disableBuiltIn bool, customPatterns []string) (*InjectionScanner, error) {
	var sources []string

	if !disableBuiltIn {
		sources = append(sources, builtInInjectionPatterns...)
	}
	sources = append(sources, customPatterns...)

	compiled := make([]*regexp.Regexp, 0, len(sources))
	for _, p := range sources {
		// Prepend case-insensitive flag if not already present.
		if !strings.HasPrefix(p, "(?i)") {
			p = "(?i)" + p
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("compiling injection pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}

	return &InjectionScanner{patterns: compiled}, nil
}

func (s *InjectionScanner) Name() string { return "injection" }

func (s *InjectionScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	for _, re := range s.patterns {
		if match := re.FindString(content); match != "" {
			return ScanResult{
				Verdict:     VerdictBlock,
				Content:     content,
				Threats:     []string{fmt.Sprintf("prompt injection detected: matched pattern %q", re.String())},
				ScannerName: s.Name(),
			}, nil
		}
	}

	return ScanResult{
		Verdict:     VerdictPass,
		Content:     content,
		ScannerName: s.Name(),
	}, nil
}
