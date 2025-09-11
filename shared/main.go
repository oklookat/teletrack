package shared

import (
	"fmt"
	"html"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
)

var _emoticonsUTF = []string{
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

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// TotalRandomEmoji returns either a random UTF emoticon or 3 standard emojis
func TotalRandomEmoji() string {
	if rng.Intn(2) == 1 {
		return RandomEmoticonUTF()
	}
	return fmt.Sprintf("%s %s %s", RandomEmoji(), RandomEmoji(), RandomEmoji())
}

// RandomEmoticonUTF returns a single random UTF emoticon
func RandomEmoticonUTF() string {
	return _emoticonsUTF[rng.Intn(len(_emoticonsUTF))]
}

// RandomEmoji returns a random emoji from predefined ranges
func RandomEmoji() string {
	emojiRanges := [][]int{
		{128513, 128591}, // Emoticons
		{128640, 128704}, // Transport & map symbols
	}

	r := emojiRanges[rng.Intn(len(emojiRanges))]
	min, max := r[0], r[1]
	codepoint := rng.Intn(max-min+1) + min
	return html.UnescapeString("&#" + strconv.Itoa(codepoint) + ";")
}

func TgText(text string) string {
	return bot.EscapeMarkdownUnescaped(text)
}

func TgLink(description, link string) string {
	return fmt.Sprintf("[%s](%s)", bot.EscapeMarkdownUnescaped(description), bot.EscapeMarkdownUnescaped(link))
}

func TimeToRu(t time.Time) string {
	location, _ := time.LoadLocation("Europe/Moscow")
	tInLocation := t.In(location)
	return tInLocation.Format("15:04 02.01.2006") + fmt.Sprintf(" (%s)", GetTimeZone())
}

func TimeToRuWithSeconds(t time.Time) string {
	location, _ := time.LoadLocation("Europe/Moscow")
	tInLocation := t.In(location)
	return tInLocation.Format("15:04:05 02.01.2006") + " (MSK)"
}

func GetTimeZone() string {
	zone, _ := time.Now().Zone()
	return zone
}

func EscapeMarkdownV2(input string) string {
	re := regexp.MustCompile(`([_*\[\]()~>#+\-=\|{}.!\\])`)
	return re.ReplaceAllString(input, `\$1`)
}
