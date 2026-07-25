package telegram

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
)

func tgText(text string) string {
	return bot.EscapeMarkdownUnescaped(text)
}

// SanitizeCodeSpan prepares string to put inside `code span`
func sanitizeCodeSpan(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}

// TgLink builds a MarkdownV2 link. The description is escaped but the URL is left raw
// because escaping the URL may break the link (Telegram accepts raw URLs inside parentheses).
func tgLink(description, link string) string {
	// Only ( and ) can break the MarkdownV2 link syntax inside the URL part.
	// Full escaping would break the URL itself, so we escape only these two.
	link = strings.ReplaceAll(link, `(`, `\(`)
	link = strings.ReplaceAll(link, `)`, `\)`)
	return fmt.Sprintf("[%s](%s)", bot.EscapeMarkdownUnescaped(description), link)
}

// TimeToRu formats time in Moscow timezone (MSK). Falls back to local time if loading the location fails.
func timeToRu(t time.Time) string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.Local
	}
	tInLocation := t.In(loc)
	return tInLocation.Format("15:04 02.01.2006") + fmt.Sprintf(" (%s)", getTimeZone())
}

// TimeToRuWithSeconds formats time in Moscow timezone with seconds. Falls back to local time if loading the location fails.
func timeToRuWithSeconds(t time.Time) string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.Local
	}
	tInLocation := t.In(loc)
	return tInLocation.Format("15:04:05 02.01.2006") + " (MSK)"
}

// GetTimeZone returns the current system timezone abbreviation.
func getTimeZone() string {
	zone, _ := time.Now().Zone()
	return zone
}

// precompile regex for markdown V2 escaping
var escapeMdV2Re = regexp.MustCompile(`([_*\[\]()~>#+=\|{}.!\\])`)

// EscapeMarkdownV2 escapes characters that must be escaped for Telegram MarkdownV2.
// We precompiled the regex above for performance.
func escapeMarkdownV2(input string) string {
	return escapeMdV2Re.ReplaceAllString(input, `\\$1`)
}
