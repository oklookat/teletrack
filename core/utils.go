package core

import (
	"fmt"
	"html"
	"math/rand"
	"strconv"
	"sync"
	"time"
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
)

// TotalRandomEmoji returns either a random UTF emoticon or 3 standard emojis.
// It is safe for concurrent use.
func totalRandomEmoji() string {
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
