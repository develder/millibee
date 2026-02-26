package optimizer

import "regexp"

// fillerPattern represents a compiled regex for matching filler text.
type fillerPattern struct {
	re       *regexp.Regexp
	category string
}

// defaultPatterns returns the compiled filler patterns for EN and NL.
// Patterns use word boundaries (\b) where appropriate to avoid matching
// inside identifiers (e.g. "basically_connected" should not be affected).
func defaultPatterns() []fillerPattern {
	raw := []struct {
		pattern  string
		category string
	}{
		// EN pleasantries
		{`(?i)\bcould you please\b`, "pleasantry"},
		{`(?i)\bwould you mind\b`, "pleasantry"},
		{`(?i)\bI was wondering if\b`, "pleasantry"},
		{`(?i)\bwould you be able to\b`, "pleasantry"},
		{`(?i)\bif you don'?t mind\b`, "pleasantry"},

		// NL pleasantries
		{`(?i)\bzou je alsjeblieft\b`, "pleasantry"},
		{`(?i)\bkun je misschien\b`, "pleasantry"},
		{`(?i)\bzou je eventueel\b`, "pleasantry"},

		// EN hedging
		{`(?i)\bI think maybe\b`, "hedging"},
		{`(?i)\bperhaps you could\b`, "hedging"},
		{`(?i)\bit might be good to\b`, "hedging"},

		// NL hedging
		{`(?i)\bik denk misschien\b`, "hedging"},
		{`(?i)\bmisschien zou je\b`, "hedging"},
		{`(?i)\bhet lijkt mij\b`, "hedging"},

		// EN preambles
		{`(?i)\bI have a question about\b`, "preamble"},
		{`(?i)\bso basically\b`, "preamble"},
		{`(?i)\bthe thing is\b`, "preamble"},

		// NL preambles
		{`(?i)\bik heb een vraag over\b`, "preamble"},
		{`(?i)\bhet punt is\b`, "preamble"},
		{`(?i)\bdus eigenlijk\b`, "preamble"},

		// EN verbose verbs
		{`(?i)\bI would like you to\b`, "verbose_verb"},
		{`(?i)\bI need you to\b`, "verbose_verb"},
		{`(?i)\bI want you to\b`, "verbose_verb"},

		// NL verbose verbs
		{`(?i)\bik zou graag willen dat\b`, "verbose_verb"},
		{`(?i)\bhet zou fijn zijn als\b`, "verbose_verb"},

		// EN filler words (word boundary to avoid matching inside identifiers)
		{`(?i)(?:^|\s)\bactually\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\bjust\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\breally\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\bbasically\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\bessentially\b(?:\s|$)`, "filler"},

		// NL filler words
		{`(?i)(?:^|\s)\beigenlijk\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\bgewoon\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\bdus ja\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\beven\b(?:\s|$)`, "filler"},
		{`(?i)(?:^|\s)\bsowieso\b(?:\s|$)`, "filler"},
	}

	patterns := make([]fillerPattern, 0, len(raw))
	for _, r := range raw {
		patterns = append(patterns, fillerPattern{
			re:       regexp.MustCompile(r.pattern),
			category: r.category,
		})
	}
	return patterns
}
