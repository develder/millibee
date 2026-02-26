package optimizer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	o := New()
	assert.NotNil(t, o)
	assert.False(t, o.IsEnabled(), "optimizer should be disabled by default")
}

func TestSetEnabled(t *testing.T) {
	o := New()
	o.SetEnabled(true)
	assert.True(t, o.IsEnabled())
	o.SetEnabled(false)
	assert.False(t, o.IsEnabled())
}

func TestOptimize_DisabledReturnsUnchanged(t *testing.T) {
	o := New()
	result := o.Optimize("could you please help me with this")
	assert.False(t, result.WasChanged, "should not optimize when disabled")
	assert.Equal(t, result.Original, result.Optimized)
}

func TestOptimize_EmptyInput(t *testing.T) {
	o := New()
	o.SetEnabled(true)
	result := o.Optimize("")
	assert.False(t, result.WasChanged)
	assert.Equal(t, "", result.Optimized)
}

func TestOptimize_ShortPromptUnchanged(t *testing.T) {
	o := New()
	o.SetEnabled(true)
	result := o.Optimize("could you please fix the bug")
	assert.False(t, result.WasChanged, "short prompts (<10 words) should not be optimized")
	assert.Equal(t, "could you please fix the bug", result.Optimized)
}

// --- English filler removal ---

func TestOptimize_EN_Pleasantries(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	tests := []struct {
		input    string
		contains string // should NOT contain after optimization
	}{
		{"Could you please help me refactor this function to be more readable and maintainable", "could you please"},
		{"Would you mind taking a look at this code and telling me what you think about it", "would you mind"},
		{"I was wondering if you could help me understand how this algorithm works in detail", "i was wondering if"},
	}

	for _, tt := range tests {
		result := o.Optimize(tt.input)
		assert.True(t, result.WasChanged, "should optimize: %s", tt.input)
		assert.NotContains(t, result.Optimized, tt.contains)
		assert.Greater(t, result.Saved, 0)
	}
}

func TestOptimize_EN_Hedging(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("I think maybe we should consider refactoring this entire module to use interfaces instead of concrete types")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "I think maybe")
}

func TestOptimize_EN_Preambles(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("I have a question about the way this function handles errors and returns values to the caller")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "I have a question about")
}

func TestOptimize_EN_FillerWords(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("So basically I actually really just need to essentially understand how this thing works in the codebase")
	assert.True(t, result.WasChanged)
	assert.Contains(t, result.Optimized, "understand")
	assert.Contains(t, result.Optimized, "works")
}

func TestOptimize_EN_VerboseVerbs(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("I would like you to refactor the authentication module and add proper error handling throughout the codebase")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "I would like you to")
	assert.Contains(t, result.Optimized, "refactor")
}

// --- Dutch filler removal ---

func TestOptimize_NL_Pleasantries(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	tests := []struct {
		input    string
		contains string
	}{
		{"Zou je alsjeblieft kunnen kijken naar deze code en me vertellen wat er mis is", "zou je alsjeblieft"},
		{"Kun je misschien helpen met het refactoren van deze functie zodat het leesbaarder wordt", "kun je misschien"},
	}

	for _, tt := range tests {
		result := o.Optimize(tt.input)
		assert.True(t, result.WasChanged, "should optimize: %s", tt.input)
		assert.NotContains(t, result.Optimized, tt.contains)
	}
}

func TestOptimize_NL_Hedging(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("Ik denk misschien dat we deze module beter zouden kunnen herschrijven met een andere aanpak voor errors")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "Ik denk misschien")
}

func TestOptimize_NL_Preambles(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("Ik heb een vraag over hoe deze functie omgaat met foutmeldingen en return values naar de caller")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "Ik heb een vraag over")
}

func TestOptimize_NL_FillerWords(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("Dus eigenlijk moet ik gewoon even sowieso begrijpen hoe dit systeem werkt in de huidige codebase")
	assert.True(t, result.WasChanged)
	assert.Contains(t, result.Optimized, "begrijpen")
	assert.Contains(t, result.Optimized, "werkt")
}

func TestOptimize_NL_VerboseVerbs(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("Ik zou graag willen dat je de authenticatie module refactort en error handling toevoegt aan de hele codebase")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "Ik zou graag willen dat")
	assert.Contains(t, result.Optimized, "refactort")
}

// --- Edge cases ---

func TestOptimize_WhitespaceNormalization(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("Could you please   help   me   with   this   really   important   function   that   needs   refactoring   now")
	assert.True(t, result.WasChanged)
	assert.NotContains(t, result.Optimized, "  ", "should not contain double spaces")
}

func TestOptimize_CodeBlocksPreserved(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	input := "Could you please look at this code and tell me what is wrong with the implementation:\n```go\nfunc basically() {\n\t// I was wondering if this actually works\n\tfmt.Println(\"just a test\")\n}\n```\nI think maybe there is a bug somewhere in the function above"
	result := o.Optimize(input)

	// Code block content should be preserved exactly
	assert.Contains(t, result.Optimized, "func basically()")
	assert.Contains(t, result.Optimized, "// I was wondering if this actually works")
	assert.Contains(t, result.Optimized, "\"just a test\"")
}

func TestOptimize_ResultFields(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	input := "Could you please help me understand how this really complex authentication system works in the codebase"
	result := o.Optimize(input)

	assert.Equal(t, input, result.Original)
	assert.NotEqual(t, input, result.Optimized)
	assert.True(t, result.WasChanged)
	assert.Equal(t, len(input)-len(result.Optimized), result.Saved)
	assert.Greater(t, result.Saved, 0)
}

func TestOptimize_CaseInsensitive(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	result := o.Optimize("COULD YOU PLEASE help me understand how this really important system works across the entire codebase")
	assert.True(t, result.WasChanged)
}

func TestOptimize_NoFalsePositives(t *testing.T) {
	o := New()
	o.SetEnabled(true)

	// Technical content that happens to contain filler-like words should not be mangled
	input := "The function should actually return an error when the basically_connected flag is false and the session is just_started"
	result := o.Optimize(input)

	// Words like "actually" might get stripped but the technical identifiers must survive
	assert.Contains(t, result.Optimized, "basically_connected")
	assert.Contains(t, result.Optimized, "just_started")
}
