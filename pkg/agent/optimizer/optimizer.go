package optimizer

import (
	"regexp"
	"strings"
)

// Optimizer applies rule-based prompt compression to strip filler words
// and normalize whitespace before sending prompts to the LLM.
type Optimizer struct {
	enabled  bool
	patterns []fillerPattern
}

// Result holds the outcome of an optimization pass.
type Result struct {
	Original   string
	Optimized  string
	Saved      int
	WasChanged bool
}

// minWords is the minimum word count for optimization to kick in.
const minWords = 10

var multiSpaceRe = regexp.MustCompile(`\s{2,}`)

// New creates a new Optimizer (disabled by default).
func New() *Optimizer {
	return &Optimizer{
		patterns: defaultPatterns(),
	}
}

// SetEnabled enables or disables the optimizer.
func (o *Optimizer) SetEnabled(enabled bool) {
	o.enabled = enabled
}

// IsEnabled returns whether the optimizer is enabled.
func (o *Optimizer) IsEnabled() bool {
	return o.enabled
}

// Optimize applies filler removal and whitespace normalization.
// Code blocks (``` ... ```) are preserved untouched.
// Short prompts (<20 words) are returned as-is.
func (o *Optimizer) Optimize(input string) Result {
	result := Result{Original: input, Optimized: input}

	if !o.enabled || input == "" {
		return result
	}

	// Skip short prompts
	if len(strings.Fields(input)) < minWords {
		return result
	}

	// Split into code blocks and text segments
	segments := splitCodeBlocks(input)
	var out strings.Builder
	for _, seg := range segments {
		if seg.isCode {
			out.WriteString(seg.text)
		} else {
			out.WriteString(o.optimizeText(seg.text))
		}
	}

	optimized := strings.TrimSpace(out.String())
	// Normalize whitespace
	optimized = multiSpaceRe.ReplaceAllString(optimized, " ")

	if optimized != input {
		result.Optimized = optimized
		result.Saved = len(input) - len(optimized)
		result.WasChanged = true
	}

	return result
}

// optimizeText applies filler patterns to a non-code text segment.
func (o *Optimizer) optimizeText(text string) string {
	for _, p := range o.patterns {
		text = p.re.ReplaceAllString(text, " ")
	}
	return text
}

// segment represents a piece of text, either code or prose.
type segment struct {
	text   string
	isCode bool
}

// splitCodeBlocks splits input into alternating prose and code block segments.
// Code blocks are delimited by ``` (with optional language tag).
func splitCodeBlocks(input string) []segment {
	const fence = "```"
	var segments []segment
	rest := input
	for {
		idx := strings.Index(rest, fence)
		if idx < 0 {
			if rest != "" {
				segments = append(segments, segment{text: rest, isCode: false})
			}
			break
		}
		// Text before fence
		if idx > 0 {
			segments = append(segments, segment{text: rest[:idx], isCode: false})
		}
		// Find closing fence
		afterOpen := rest[idx+len(fence):]
		closeIdx := strings.Index(afterOpen, fence)
		if closeIdx < 0 {
			// Unclosed code block — treat rest as code
			segments = append(segments, segment{text: rest[idx:], isCode: true})
			break
		}
		// Include both fences
		codeEnd := idx + len(fence) + closeIdx + len(fence)
		segments = append(segments, segment{text: rest[idx:codeEnd], isCode: true})
		rest = rest[codeEnd:]
	}
	return segments
}
