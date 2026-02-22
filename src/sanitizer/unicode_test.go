package sanitizer

import (
	"context"
	"strings"
	"testing"
)

func TestUnicodeScanner_CleanText(t *testing.T) {
	s := UnicodeScanner{}
	res, err := s.Scan(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}

func TestUnicodeScanner_PreservesWhitespace(t *testing.T) {
	s := UnicodeScanner{}
	input := "line1\nline2\ttab\rcarriage"
	res, err := s.Scan(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != input {
		t.Errorf("content = %q, want %q", res.Content, input)
	}
}

func TestUnicodeScanner_RemovesZeroWidthChars(t *testing.T) {
	s := UnicodeScanner{}
	// Zero-width space, zero-width joiner, zero-width non-joiner
	input := "hello\u200B\u200C\u200Dworld"
	res, err := s.Scan(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	if res.Content != "helloworld" {
		t.Errorf("content = %q, want %q", res.Content, "helloworld")
	}
}

func TestUnicodeScanner_RemovesBOM(t *testing.T) {
	s := UnicodeScanner{}
	input := "\uFEFFhello"
	res, err := s.Scan(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	if strings.Contains(res.Content, "\uFEFF") {
		t.Error("BOM should be removed")
	}
}

func TestUnicodeScanner_RemovesDirectionalMarks(t *testing.T) {
	s := UnicodeScanner{}
	// Right-to-left mark, left-to-right mark
	input := "hello\u200Fworld\u200E"
	res, err := s.Scan(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	if res.Content != "helloworld" {
		t.Errorf("content = %q, want %q", res.Content, "helloworld")
	}
}

func TestUnicodeScanner_NormalizesNFKC(t *testing.T) {
	s := UnicodeScanner{}
	// ﬁ (U+FB01, fi ligature) should normalize to "fi"
	input := "de\uFB01ne"
	res, err := s.Scan(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "define" {
		t.Errorf("content = %q, want %q", res.Content, "define")
	}
}

func TestUnicodeScanner_EmptyString(t *testing.T) {
	s := UnicodeScanner{}
	res, err := s.Scan(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.Verdict)
	}
}
