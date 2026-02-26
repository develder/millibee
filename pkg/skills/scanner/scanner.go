package scanner

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const defaultMaxFileSize = 1 << 20 // 1MB

// Scanner performs local heuristic scanning of extracted skill directories.
type Scanner struct {
	patterns    []Pattern
	thresholds  Thresholds
	maxFileSize int64
}

// New creates a Scanner with the default pattern set and thresholds.
func New() *Scanner {
	return &Scanner{
		patterns:    DefaultPatterns(),
		thresholds:  DefaultThresholds(),
		maxFileSize: defaultMaxFileSize,
	}
}

// NewWithOptions creates a Scanner with custom thresholds.
func NewWithOptions(t Thresholds) *Scanner {
	return &Scanner{
		patterns:    DefaultPatterns(),
		thresholds:  t,
		maxFileSize: defaultMaxFileSize,
	}
}

// ScanDirectory scans all files in skillDir and returns a ScanReport.
func (s *Scanner) ScanDirectory(skillDir string) (*ScanReport, error) {
	report := &ScanReport{
		Findings: []Finding{},
	}

	err := filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}

		name := d.Name()

		// Skip hidden files and directories.
		if strings.HasPrefix(name, ".") && name != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		// Check file size.
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > s.maxFileSize {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Check binary files for bundled executables, then skip them.
		if isBinary(content) {
			relPath, relErr := filepath.Rel(skillDir, path)
			if relErr != nil {
				relPath = name
			}
			relPath = filepath.ToSlash(relPath)
			if f := detectBinaryType(relPath, content); f != nil {
				report.Findings = append(report.Findings, *f)
			}
			report.FilesScanned++
			return nil
		}

		relPath, err := filepath.Rel(skillDir, path)
		if err != nil {
			relPath = name
		}
		// Normalize path separators for consistent output.
		relPath = filepath.ToSlash(relPath)

		findings := s.ScanContent(relPath, content)
		report.Findings = append(report.Findings, findings...)
		report.FilesScanned++

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Calculate score and verdict.
	for _, f := range report.Findings {
		report.TotalScore += int(f.Severity)
	}

	switch {
	case report.TotalScore >= s.thresholds.BlockAt:
		report.Verdict = VerdictBlock
	case report.TotalScore >= s.thresholds.WarnAt:
		report.Verdict = VerdictWarn
	default:
		report.Verdict = VerdictAllow
	}

	return report, nil
}

// ScanContent scans a single piece of content and returns all findings.
func (s *Scanner) ScanContent(filename string, content []byte) []Finding {
	var findings []Finding

	// Build a line offset index for line number lookups.
	lineOffsets := buildLineOffsets(content)

	for _, p := range s.patterns {
		matches := p.Regex.FindAllIndex(content, -1)
		for _, loc := range matches {
			line := offsetToLine(lineOffsets, loc[0])
			matched := string(content[loc[0]:loc[1]])
			if len(matched) > 120 {
				matched = matched[:120]
			}

			findings = append(findings, Finding{
				File:        filename,
				Line:        line,
				Category:    p.Category,
				PatternName: p.Name,
				Match:       matched,
				Severity:    p.Severity,
			})
		}
	}

	return findings
}

// isBinary checks if content appears to be binary by looking for null bytes
// in the first 512 bytes.
func isBinary(content []byte) bool {
	check := content
	if len(check) > 512 {
		check = check[:512]
	}
	return bytes.ContainsRune(check, 0)
}

// detectBinaryType checks if a binary file is a known executable format.
func detectBinaryType(relPath string, content []byte) *Finding {
	header := content
	if len(header) > 4 {
		header = header[:4]
	}

	var name string
	switch {
	case len(header) >= 4 && header[0] == 0x7f && header[1] == 'E' && header[2] == 'L' && header[3] == 'F':
		name = "ELF executable"
	case len(header) >= 2 && header[0] == 'M' && header[1] == 'Z':
		name = "Windows PE executable"
	case len(header) >= 4 && header[0] == 0xfe && header[1] == 0xed && header[2] == 0xfa && (header[3] == 0xce || header[3] == 0xcf):
		name = "Mach-O executable"
	case len(header) >= 4 && header[0] == 0xcf && header[1] == 0xfa && header[2] == 0xed && header[3] == 0xfe:
		name = "Mach-O executable (reverse byte order)"
	default:
		return nil
	}

	return &Finding{
		File:        relPath,
		Line:        1,
		Category:    "bundled_binary",
		PatternName: name,
		Match:       name,
		Severity:    SeverityHigh,
	}
}

// buildLineOffsets returns the byte offset of the start of each line.
func buildLineOffsets(content []byte) []int {
	offsets := []int{0}
	for i, b := range content {
		if b == '\n' && i+1 < len(content) {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// offsetToLine returns the 1-based line number for a byte offset.
func offsetToLine(lineOffsets []int, offset int) int {
	// Binary search for the line.
	lo, hi := 0, len(lineOffsets)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if lineOffsets[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return lo // 1-based because offsets[0]=0 maps to line 1
}
