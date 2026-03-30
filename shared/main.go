package shared

import (
	"fmt"
	"html"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
)

// NOTE: keep a mutex because math/rand.Rand is NOT safe for concurrent use.
var (
	_emoticonsUTF = []string{
		":)", ":3", "¯\\_(ツ)_/¯", "( ͡°͜ʖ ͡°)", "-_-", ":(", ":D", ":P",
		"XD", "(>_<)", ";)", "T_T", "UwU", "OwO", ":|", ":v", "(^_^)",
		"(•‿•)", "(¬_¬)", "o_O", "O_o", "(╯°□°）╯︵ ┻━┻", "(^o^)", ":')",
		":*", ":^)", ":>", ">:(", ">:3", "<3", "</3", "(>‿<)", "(´• ω •`)",
		"(｡♥‿♥｡)", "(╥﹏╥)", "ヽ(´▽`)/", "(^_^)/", "(^.^)/", "(^3^)/",
		"(*^_^*)", "(^_~)", "(≧∇≦)", "(¬‿¬)", "(°ロ°)☝", "(•‿•)✌",
		"(^ω^)", "(^з^)-☆", "(^_^*)", "(^.^*)", ":o)", ":]", ":}", "B)",
		":S", ":$", ":O", ":/", ":\\", ":X", ">:|", "0_0", "(´•̥ ̯ •̥`)",
		"(๑>ᴗ<๑)", "(╯°□°)╯", "(ง'̀-'́)ง", "ヽ(；▽；)ノ", "ヽ(´ー｀)ノ",
		"(￣▽￣)ノ", "(´• ω •`)", ">:D", ":-]", ":-)", ":-(", ":-P", ":o",
		"ヽ(´∇｀)ﾉ", "(⌒‿⌒)", "(^_^)b", "(•‿•)ノ", "(^.^)v", "(=^.^=)",
		"(•ε•)", "(´･ω･`)", "(^～^)", "(^.^)/~~", "(^_^)ノ", "(✧ω✧)",
		"(◕‿◕✿)", "(｡◕‿◕｡)", "(≧◡≦)", "(≧ω≦)", "(⌒▽⌒)", "(*≧ω≦)",
		"(´▽`)", "(´∇｀)", "(•‿•)♡", "(*^.^*)", "(￣ω￣)", "(＾▽＾)",
		"(*≧▽≦)", "(^･o･^)ﾉ”", "(^・ω・^)", "(⌒_⌒;)", "(´•̥ω•̥`)",
	}
	rng   = rand.New(rand.NewSource(time.Now().UnixNano()))
	rngMu sync.Mutex

	// precompile regex for markdown V2 escaping
	escapeMdV2Re = regexp.MustCompile(`([_*\[\]()~>#+=\|{}.!\\])`)
)

// TotalRandomEmoji returns either a random UTF emoticon or 3 standard emojis.
// It is safe for concurrent use.
func TotalRandomEmoji() string {
	if randBool() {
		return RandomEmoticonUTF()
	}
	return fmt.Sprintf("%s %s %s", RandomEmoji(), RandomEmoji(), RandomEmoji())
}

// RandomEmoticonUTF returns a single random UTF emoticon. Concurrent-safe.
func RandomEmoticonUTF() string {
	rngMu.Lock()
	idx := rng.Intn(len(_emoticonsUTF))
	rngMu.Unlock()
	return _emoticonsUTF[idx]
}

// RandomEmoji returns a random emoji from predefined ranges. Concurrent-safe.
func RandomEmoji() string {
	emojiRanges := [][]int{
		{128513, 128591}, // Emoticons
		{128640, 128704}, // Transport & map symbols (subset)
	}

	rngMu.Lock()
	r := emojiRanges[rng.Intn(len(emojiRanges))]
	min, max := r[0], r[1]
	codepoint := rng.Intn(max-min+1) + min
	rngMu.Unlock()

	// numeric entity -> unescape
	return html.UnescapeString("&#" + strconv.Itoa(codepoint) + ";")
}

// randBool returns a random boolean (50/50). Concurrent-safe.
func randBool() bool {
	rngMu.Lock()
	b := rng.Intn(2) == 1
	rngMu.Unlock()
	return b
}

// TgText escapes text for Telegram Markdown V2 using the bot helper.
func TgText(text string) string {
	return bot.EscapeMarkdownUnescaped(text)
}

// SanitizeCodeSpan prepares string to put inside `code span`
func SanitizeCodeSpan(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}

// TgLink builds a MarkdownV2 link. The description is escaped but the URL is left raw
// because escaping the URL may break the link (Telegram accepts raw URLs inside parentheses).
func TgLink(description, link string) string {
	// Only ( and ) can break the MarkdownV2 link syntax inside the URL part.
	// Full escaping would break the URL itself, so we escape only these two.
	link = strings.ReplaceAll(link, `(`, `\(`)
	link = strings.ReplaceAll(link, `)`, `\)`)
	return fmt.Sprintf("[%s](%s)", bot.EscapeMarkdownUnescaped(description), link)
}

// TimeToRu formats time in Moscow timezone (MSK). Falls back to local time if loading the location fails.
func TimeToRu(t time.Time) string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.Local
	}
	tInLocation := t.In(loc)
	return tInLocation.Format("15:04 02.01.2006") + fmt.Sprintf(" (%s)", GetTimeZone())
}

// TimeToRuWithSeconds formats time in Moscow timezone with seconds. Falls back to local time if loading the location fails.
func TimeToRuWithSeconds(t time.Time) string {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.Local
	}
	tInLocation := t.In(loc)
	return tInLocation.Format("15:04:05 02.01.2006") + " (MSK)"
}

// GetTimeZone returns the current system timezone abbreviation.
func GetTimeZone() string {
	zone, _ := time.Now().Zone()
	return zone
}

// EscapeMarkdownV2 escapes characters that must be escaped for Telegram MarkdownV2.
// We precompiled the regex above for performance.
func EscapeMarkdownV2(input string) string {
	return escapeMdV2Re.ReplaceAllString(input, `\\$1`)
}
