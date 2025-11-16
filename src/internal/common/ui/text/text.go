package text

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	titleCaser = cases.Title(language.English)

	// Common words usually not capitalized in English titles
	minorWords = map[string]bool{
		"a": true, "an": true, "and": true, "as": true, "at": true,
		"but": true, "by": true, "for": true, "in": true, "nor": true,
		"of": true, "on": true, "or": true, "per": true, "the": true,
		"to": true, "vs": true, "via": true, "with": true,
	}
)

// TitleSmart applies title casing to the given string, following common
// English title capitalization rules. Minor words are left lowercase
// unless they appear at the beginning or end.
func TitleSmart(s string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}

	for i, w := range words {
		lower := strings.ToLower(w)
		if i == 0 || i == len(words)-1 || !minorWords[lower] {
			words[i] = titleCaser.String(lower)
		} else {
			words[i] = lower
		}
	}

	return strings.Join(words, " ")
}

func Title(s string) string {
	return titleCaser.String(s)
}
