package telegram

import (
	"fmt"
	"log/slog"
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

var _moscowLoc = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		slog.Error("failed to load timezone", "error", err.Error())
		return time.Local
	}
	return loc
}()

// clockEmojis maps hour (0–11) → [full-hour emoji, half-hour emoji]
var clockEmojis = [12][2]string{
	{"🕛", "🕧"}, // 0 / 12
	{"🕐", "🕜"}, // 1
	{"🕑", "🕝"}, // 2
	{"🕒", "🕞"}, // 3
	{"🕓", "🕟"}, // 4
	{"🕔", "🕠"}, // 5
	{"🕕", "🕡"}, // 6
	{"🕖", "🕢"}, // 7
	{"🕗", "🕣"}, // 8
	{"🕘", "🕤"}, // 9
	{"🕙", "🕥"}, // 10
	{"🕚", "🕦"}, // 11
}

// clockEmoji returns the Unicode clock-face emoji closest to the given time,
// rounding to the nearest 30-minute slot (with wraparound across hours/day).
func clockEmoji(t time.Time) string {
	h := t.Hour() % 12
	m := t.Minute()

	// Total 30-minute slots since 12:00, rounded to nearest (ties round up).
	slot := (h*60 + m + 15) / 30
	slot %= 24

	hourIdx := (slot / 2) % 12
	half := slot % 2

	return clockEmojis[hourIdx][half]
}

func timeToRuWithSeconds(t time.Time) string {
	return t.In(_moscowLoc).Format("15:04:05 02.01.2006") + " (MSK)"
}

func timeToRuWithSecondsClockEmoji(t time.Time) string {
	return clockEmoji(t) + " " + timeToRuWithSeconds(t)
}
