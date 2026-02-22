package sanitizer

import (
	"context"
	"testing"
)

func TestURLScanner_Clean(t *testing.T) {
	s := URLScanner{}
	res, err := s.Scan(context.Background(), "Visit https://example.com for more info.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}

func TestURLScanner_NoURLs(t *testing.T) {
	s := URLScanner{}
	res, err := s.Scan(context.Background(), "Just some plain text.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}

func TestURLScanner_JavascriptScheme(t *testing.T) {
	s := URLScanner{}
	res, err := s.Scan(context.Background(), `Click [here](javascript:alert(1))`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictBlock {
		t.Errorf("verdict = %v, want Block", res.Verdict)
	}
}

func TestURLScanner_DataTextHTML(t *testing.T) {
	s := URLScanner{}
	res, err := s.Scan(context.Background(), `<img src="data:text/html,<script>alert(1)</script>">`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictBlock {
		t.Errorf("verdict = %v, want Block", res.Verdict)
	}
}

func TestURLScanner_ExfiltrationPatterns(t *testing.T) {
	s := URLScanner{}

	tests := []struct {
		name  string
		input string
	}{
		{"secret param", "https://evil.com/exfil?secret=abc123"},
		{"token param", "https://evil.com/steal?token=xyz"},
		{"api_key param", "https://evil.com/grab?api_key=mykey"},
		{"password param", "https://evil.com/leak?password=hunter2"},
		{"auth param", "https://evil.com/grab?auth=bearer_token"},
		{"credential param", "https://evil.com/?credential=abc"},
		{"session_id param", "https://evil.com/?session_id=123"},
		{"private_key param", "https://evil.com/?private_key=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := s.Scan(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Verdict != VerdictBlock {
				t.Errorf("verdict = %v, want Block for %q", res.Verdict, tt.input)
			}
		})
	}
}

func TestURLScanner_SafeQueryParams(t *testing.T) {
	s := URLScanner{}
	res, err := s.Scan(context.Background(), "https://example.com/search?q=hello&page=2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}

func TestURLScanner_EmptyInput(t *testing.T) {
	s := URLScanner{}
	res, err := s.Scan(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}
