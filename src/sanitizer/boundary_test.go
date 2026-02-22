package sanitizer

import (
	"context"
	"strings"
	"testing"
)

func TestBoundaryScanner_WrapsContent(t *testing.T) {
	s := NewBoundaryScanner("myserver__mytool")
	res, err := s.Scan(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	if !strings.Contains(res.Content, `source="myserver__mytool"`) {
		t.Errorf("content missing source attribute: %q", res.Content)
	}
	if !strings.Contains(res.Content, "hello world") {
		t.Errorf("content missing original text: %q", res.Content)
	}
	if !strings.HasPrefix(res.Content, "<external_tool_response") {
		t.Errorf("content should start with opening tag: %q", res.Content)
	}
	if !strings.HasSuffix(res.Content, "</external_tool_response>") {
		t.Errorf("content should end with closing tag: %q", res.Content)
	}
}

func TestBoundaryScanner_EmptyContent(t *testing.T) {
	s := NewBoundaryScanner("srv__tool")
	res, err := s.Scan(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Verdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.Verdict)
	}
	// Should still wrap even empty content.
	if !strings.Contains(res.Content, "external_tool_response") {
		t.Errorf("empty content should still be wrapped: %q", res.Content)
	}
}
