package sanitizer

import "context"

// Pipeline executes an ordered sequence of Scanners against content.
// On VerdictBlock it short-circuits. On VerdictModify it threads the
// modified content into subsequent scanners.
type Pipeline struct {
	scanners []Scanner
}

// NewPipeline creates a pipeline from the given scanners. Execution
// order matches the slice order.
func NewPipeline(scanners ...Scanner) *Pipeline {
	return &Pipeline{scanners: scanners}
}

// Process runs all scanners in order and returns an aggregated result.
func (p *Pipeline) Process(ctx context.Context, content string) (PipelineResult, error) {
	current := content
	result := PipelineResult{
		FinalVerdict: VerdictPass,
		ScanResults:  make([]ScanResult, 0, len(p.scanners)),
	}

	for _, s := range p.scanners {
		sr, err := s.Scan(ctx, current)
		if err != nil {
			return result, err
		}

		result.ScanResults = append(result.ScanResults, sr)
		result.AllThreats = append(result.AllThreats, sr.Threats...)

		switch sr.Verdict {
		case VerdictBlock:
			result.FinalVerdict = VerdictBlock
			result.FinalContent = sr.Content
			return result, nil
		case VerdictModify:
			if result.FinalVerdict != VerdictBlock {
				result.FinalVerdict = VerdictModify
			}
			current = sr.Content
		default:
			// VerdictPass — keep current content as-is
		}
	}

	result.FinalContent = current
	return result, nil
}
