package scanner

// Severity represents how dangerous a detected pattern is.
type Severity int

const (
	SeverityLow      Severity = 1
	SeverityMedium   Severity = 5
	SeverityHigh     Severity = 10
	SeverityCritical Severity = 20
)

// Verdict is the final outcome of a scan.
type Verdict string

const (
	VerdictAllow Verdict = "allow"
	VerdictWarn  Verdict = "warn"
	VerdictBlock Verdict = "block"
)

// Finding represents a single detected dangerous pattern.
type Finding struct {
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Category    string   `json:"category"`
	PatternName string   `json:"pattern_name"`
	Match       string   `json:"match"`
	Severity    Severity `json:"severity"`
}

// ScanReport is the complete output of scanning a skill directory.
type ScanReport struct {
	Verdict      Verdict   `json:"verdict"`
	TotalScore   int       `json:"total_score"`
	Findings     []Finding `json:"findings"`
	FilesScanned int       `json:"files_scanned"`
}

// Thresholds configures when to warn vs block.
type Thresholds struct {
	WarnAt  int
	BlockAt int
}

// DefaultThresholds returns the default scoring thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		WarnAt:  5,
		BlockAt: 20,
	}
}
