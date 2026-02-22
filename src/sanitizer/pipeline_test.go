package sanitizer

import (
	"context"
	"errors"
	"testing"
)

// stubScanner is a test helper that returns a preconfigured result.
type stubScanner struct {
	name   string
	result ScanResult
	err    error
}

func (s stubScanner) Name() string { return s.name }
func (s stubScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	if s.err != nil {
		return ScanResult{}, s.err
	}
	r := s.result
	if r.Content == "" {
		r.Content = content
	}
	return r, nil
}

func TestPipeline_AllPass(t *testing.T) {
	p := NewPipeline(
		stubScanner{name: "a", result: ScanResult{Verdict: VerdictPass}},
		stubScanner{name: "b", result: ScanResult{Verdict: VerdictPass}},
	)

	res, err := p.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalVerdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.FinalVerdict)
	}
	if res.FinalContent != "hello" {
		t.Errorf("content = %q, want %q", res.FinalContent, "hello")
	}
	if len(res.ScanResults) != 2 {
		t.Errorf("scan results count = %d, want 2", len(res.ScanResults))
	}
}

func TestPipeline_ModifyThreadsContent(t *testing.T) {
	p := NewPipeline(
		stubScanner{name: "modifier", result: ScanResult{Verdict: VerdictModify, Content: "modified"}},
		stubScanner{name: "checker", result: ScanResult{Verdict: VerdictPass}},
	)

	res, err := p.Process(context.Background(), "original")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalVerdict != VerdictModify {
		t.Errorf("verdict = %v, want Modify", res.FinalVerdict)
	}
	if res.FinalContent != "modified" {
		t.Errorf("content = %q, want %q", res.FinalContent, "modified")
	}
}

func TestPipeline_BlockShortCircuits(t *testing.T) {
	secondRan := false
	p := NewPipeline(
		stubScanner{name: "blocker", result: ScanResult{
			Verdict: VerdictBlock,
			Content: "blocked",
			Threats: []string{"bad stuff"},
		}},
		stubScanner{name: "never", result: ScanResult{Verdict: VerdictPass}},
	)
	// Override second scanner to track execution.
	p.scanners[1] = &trackingScanner{ran: &secondRan}

	res, err := p.Process(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalVerdict != VerdictBlock {
		t.Errorf("verdict = %v, want Block", res.FinalVerdict)
	}
	if secondRan {
		t.Error("second scanner should not have run after block")
	}
	if len(res.AllThreats) != 1 || res.AllThreats[0] != "bad stuff" {
		t.Errorf("threats = %v, want [bad stuff]", res.AllThreats)
	}
	if len(res.ScanResults) != 1 {
		t.Errorf("scan results = %d, want 1 (short-circuited)", len(res.ScanResults))
	}
}

func TestPipeline_ErrorPropagates(t *testing.T) {
	scanErr := errors.New("scanner failed")
	p := NewPipeline(
		stubScanner{name: "broken", err: scanErr},
	)

	_, err := p.Process(context.Background(), "input")
	if !errors.Is(err, scanErr) {
		t.Errorf("error = %v, want %v", err, scanErr)
	}
}

func TestPipeline_ThreatsAccumulate(t *testing.T) {
	p := NewPipeline(
		stubScanner{name: "a", result: ScanResult{
			Verdict: VerdictModify,
			Content: "cleaned",
			Threats: []string{"threat-1"},
		}},
		stubScanner{name: "b", result: ScanResult{
			Verdict: VerdictModify,
			Content: "double-cleaned",
			Threats: []string{"threat-2"},
		}},
	)

	res, err := p.Process(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.AllThreats) != 2 {
		t.Errorf("threats count = %d, want 2", len(res.AllThreats))
	}
}

func TestPipeline_Empty(t *testing.T) {
	p := NewPipeline()
	res, err := p.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalVerdict != VerdictPass {
		t.Errorf("verdict = %v, want Pass", res.FinalVerdict)
	}
	if res.FinalContent != "hello" {
		t.Errorf("content = %q, want %q", res.FinalContent, "hello")
	}
}

// trackingScanner records whether Scan was called.
type trackingScanner struct {
	ran *bool
}

func (s *trackingScanner) Name() string { return "tracking" }
func (s *trackingScanner) Scan(_ context.Context, content string) (ScanResult, error) {
	*s.ran = true
	return ScanResult{Verdict: VerdictPass, Content: content}, nil
}
