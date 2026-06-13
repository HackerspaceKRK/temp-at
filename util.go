package main

import (
	"strings"
	"unicode"
)

// NormalizeName converts a string to a normalized format for usage as a label.
// It lowercases the string, replaces Polish characters with Latin equivalents,
// and replaces spaces with underscores.
func NormalizeName(s string) string {
	s = strings.ToLower(s)

	// Replace Polish characters
	replacements := map[rune]rune{
		'ą': 'a',
		'ć': 'c',
		'ę': 'e',
		'ł': 'l',
		'ń': 'n',
		'ó': 'o',
		'ś': 's',
		'ż': 'z',
		'ź': 'z',
	}

	var sb strings.Builder
	for _, r := range s {
		if replacement, ok := replacements[r]; ok {
			sb.WriteRune(replacement)
		} else if unicode.IsSpace(r) {
			sb.WriteRune('_')
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(r)
		}
		// Skip other special characters if any
	}

	return sb.String()
}

// NormalizeMac canonicalizes a MAC address to lowercase colon-separated form
// (aa:bb:cc:dd:ee:ff) so values from different sources can be matched. Inputs
// with "-" separators or no separators are accepted; unparsable input is
// lowercased and returned as-is.
func NormalizeMac(s string) string {
	hex := strings.ToLower(strings.NewReplacer(":", "", "-", "", ".", "").Replace(strings.TrimSpace(s)))
	if len(hex) != 12 {
		return strings.ToLower(strings.TrimSpace(s))
	}
	var b strings.Builder
	b.Grow(17)
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hex[i : i+2])
	}
	return b.String()
}
