package spotify

import (
	"fmt"
	"regexp"
	"strings"
)

var sentenceEnd = regexp.MustCompile(`(?m)(.*?[.!?])\s*`)

func smartTruncateSentences(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	sentences := sentenceEnd.FindAllString(text, -1)
	var sb strings.Builder
	for _, s := range sentences {
		if sb.Len()+len(s) > maxLen {
			break
		}
		sb.WriteString(s)
	}
	final := strings.TrimSpace(sb.String())
	if final == "" {
		return safeTruncate(text, maxLen)
	}
	last := final[len(final)-1]
	if last != '.' && last != '!' && last != '?' {
		final += "..."
	}
	return final
}

func safeTruncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	trimmed := text[:maxLen]
	if idx := strings.LastIndex(trimmed, " "); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed) + "..."
}

// wrapErr — small helper to keep error messages consistent
func wrapErr(ctx string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", ctx, err)
}
