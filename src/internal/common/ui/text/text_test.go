package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "Hello World"},
		{"HELLO WORLD", "Hello World"},
		{"go programming", "Go Programming"},
		{"", ""},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, Title(tt.input), "input: %q", tt.input)
	}
}

func TestTitleSmart(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic title casing",
			input:    "the quick brown fox",
			expected: "The Quick Brown Fox",
		},
		{
			name:     "minor words in the middle are lowercase",
			input:    "war and peace in europe",
			expected: "War and Peace in Europe",
		},
		{
			name:     "minor words at start and end are capitalized",
			input:    "in the end",
			expected: "In the End",
		},
		{
			name:     "handles single word",
			input:    "go",
			expected: "Go",
		},
		{
			name:     "handles multiple spaces and mixed case",
			input:    "  a tale OF two  cities ",
			expected: "A Tale of Two Cities",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "handles punctuation and vs abbreviation",
			input:    "man vs wild",
			expected: "Man vs Wild",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TitleSmart(tt.input)
			assert.Equal(t, tt.expected, got, "input: %q", tt.input)
		})
	}
}
