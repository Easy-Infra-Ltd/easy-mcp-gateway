package sanitizer

import (
	"context"
	"strings"
	"testing"
)

func TestLengthScanner_UnderLimit(t *testing.T) {
	s := NewLengthScanner(100)
	res, err := s.Scan(context.Background(), "short")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}

func TestLengthScanner_ExactLimit(t *testing.T) {
	s := NewLengthScanner(5)
	res, err := s.Scan(context.Background(), "abcde")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}

func TestLengthScanner_OverLimit(t *testing.T) {
	s := NewLengthScanner(5)
	res, err := s.Scan(context.Background(), "abcdefgh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	if !strings.HasPrefix(res.Content, "abcde") {
		t.Errorf("content should start with first 5 chars, got %q", res.Content)
	}
	if !strings.HasSuffix(res.Content, "[truncated]") {
		t.Errorf("content should end with [truncated], got %q", res.Content)
	}
}

func TestLengthScanner_MultibyteRunes(t *testing.T) {
	s := NewLengthScanner(3)
	// 4 runes: 日本語テ
	res, err := s.Scan(context.Background(), "日本語テ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	runes := []rune(strings.TrimSuffix(res.Content, "\n[truncated]"))
	if len(runes) != 3 {
		t.Errorf("rune count = %d, want 3", len(runes))
	}
}

func TestLengthScanner_EmptyString(t *testing.T) {
	s := NewLengthScanner(100)
	res, err := s.Scan(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}
