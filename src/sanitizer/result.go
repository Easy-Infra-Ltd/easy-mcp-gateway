package sanitizer

// Verdict represents the outcome of a scan.
type Verdict int

const (
	// VerdictPass means the content is clean.
	VerdictPass Verdict = iota
	// VerdictModify means the content was sanitized and should be used
	// in place of the original.
	VerdictModify
	// VerdictBlock means the content is malicious and should be rejected.
	VerdictBlock
)

func (v Verdict) String() string {
	switch v {
	case VerdictPass:
		return "pass"
	case VerdictModify:
		return "modify"
	case VerdictBlock:
		return "block"
	default:
		return "unknown"
	}
}

// ScanResult is the outcome of a single Scanner.
type ScanResult struct {
	Verdict     Verdict
	Content     string   // original or modified content
	Threats     []string // human-readable threat descriptions
	ScannerName string
}

// PipelineResult aggregates results from all scanners in a pipeline.
type PipelineResult struct {
	FinalVerdict Verdict
	FinalContent string
	AllThreats   []string
	ScanResults  []ScanResult
}
