package core

import (
	"fmt"
	"html"
	"math/rand/v2"
	"strconv"
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

// totalRandomEmoji returns either a random UTF emoticon or three random emojis.
// Safe for concurrent use (math/rand/v2 is concurrent-safe).
func totalRandomEmoji() string {
	if rand.IntN(2) == 1 {
		return RandomEmoticonUTF()
	}
	return fmt.Sprintf("%s %s %s", RandomEmoji(), RandomEmoji(), RandomEmoji())
}

// RandomEmoticonUTF returns a single random UTF emoticon.
func RandomEmoticonUTF() string {
	return _emoticonsUTF[rand.IntN(len(_emoticonsUTF))]
}

// RandomEmoji returns a random emoji from a small set of Unicode ranges.
func RandomEmoji() string {
	emojiRanges := [][2]int{
		{128513, 128591}, // Emoticons
		{128640, 128704}, // Transport & map symbols (subset)
	}
	r := emojiRanges[rand.IntN(len(emojiRanges))]
	codepoint := rand.IntN(r[1]-r[0]+1) + r[0]
	return html.UnescapeString("&#" + strconv.Itoa(codepoint) + ";")
}
