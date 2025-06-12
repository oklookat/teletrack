package shared

import (
	"fmt"
	"html"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
)

var _emoticonsUTF = []string{
	"͡° ͜ʖ ͡°",
	"ఠൠఠ )ﾉ",
	"╬ ಠ益ಠ",
	"ヽ༼ ಠ益ಠ ༽ﾉ",
	"ლ(ಠ益ಠლ)",
	"ლ(•́•́ლ)",
	"ಥ﹏ಥ",
	"◔_◔",
	"ʚ(•｀",
	"⊙.☉)7",
	"¿ⓧ_ⓧﮌ",
	"ミ●﹏☉ミ",
	"｡ﾟ( ﾟஇ‸இﾟ)ﾟ｡",
	"ಥ_ಥ",
	"༼ ༎ຶ ෴ ༎ຶ༽",
	"ʕ•ᴥ•ʔ",
	"｡◕‿◕｡",
	"ヽ( •_)ᕗ",
	"♪♪ ヽ(ˇ∀ˇ )ゞ",
	"┌(ㆆ㉨ㆆ)ʃ",
	"щ（ﾟДﾟщ）",
	"ಠ‿ಠ",
	"٩◔̯◔۶",
	"⊙﹏⊙",
	"( ಠ ʖ̯ ಠ)",
	"ᕦ(ò_óˇ)ᕤ",
	"ヾ(-_- )ゞ",
	"☜(⌒▽⌒)☞",
	"ح(•̀ж•́)ง †",
	"⥀.⥀",
	"`･ω･´",
	"V•ᴥ•V",
	"(ง̀-́)ง",
	"ლ(｀ー´ლ)",
	"ᕙ(⇀‸↼‶)ᕗ",
	"⁽⁽ଘ( ˊᵕˋ )ଓ⁾⁾",
	"ح˚௰˚づ",
	"t(-_-t)",
	"(° ͜ʖ͡°)╭∩╮",
	"ʕ •`ᴥ•´ʔ",
	"ヽ(´▽`)/",
	"\\(ᵔᵕᵔ)/",
	"(งツ)ว",
	"(づ￣ ³￣)づ",
	"(⊃｡•́‿•̀｡)⊃",
	"(҂◡_◡)",
	"ʘ‿ʘ",
	"°‿‿°",
	"{ಠʖಠ}",
	"( ఠ ͟ʖ ఠ)",
	"⊂(◉‿◉)つ",
	"( ˘ ³˘)♥",
	"ᵒᴥᵒ#",
	"◖ᵔᴥᵔ◗ ♪ ♫",
	"(._.)",
	"♥‿♥",
	"-`ღ´-",
	"¯\\(°_o)/¯",
	"ฅ^•ﻌ•^ฅ",
	"ヾ(´〇`)ﾉ♪♪♪",
	"ಠಠ",
	"(☞ﾟヮﾟ)☞",
	"ఠ_ఠ",
	"(Ծ‸ Ծ)",
	"ಠ_ಠ",
	"ᴖ̮ ̮ᴖ",
	"{•̃_•̃}",
	"ε=ε=ε=┌(;*´Д`)ﾉ",
	"(ᵟຶ︵ ᵟຶ)",
	"(ಥ⌣ಥ)",
	"(◠﹏◠)",
	"ᵔᴥᵔ",
	"( ˇ෴ˇ )",
	"(๑•́ ₃ •̀๑)",
	"눈_눈",
	"ʕʘ̅͜ʘ̅ʔ",
	"ʕᵔᴥᵔʔ",
	"٩(๏_๏)۶",
	"(づ｡◕‿‿◕｡)づ",
	"ᕕ( ᐛ )ᕗ",
	"(っ▀¯▀)つ",
	"(╯°□°）╯︵ ┻━┻",
	"(⩾﹏⩽)",
	"“ヽ(´▽｀)ノ”",
	"( ͡ಠ ʖ̯ ͡ಠ)",
	"ԅ(≖‿≖ԅ)",
	"q(❂‿❂)p",
	"~(^-^)~",
	"(っ•́｡•́)♪♬",
	"ʕ •́؈•̀)",
	"(•̀ᴗ•́)و ̑̑",
	"(∩｀-´)⊃━☆ﾟ.*･｡ﾟ",
	"´･_･`",
	"っ˘ڡ˘ς",
	"[¬º-°]¬",
	"(⊙_◎)",
	":)", ":(", ":D", ";)", ":P", ":-|", ":O", ":'(", ":3", ":*",
	">:(", ">.<", ">_<", "^_^", "-_-", "o.O", "O.o", "(¬_¬)", "(ಠ_ಠ)",
	"(ಥ﹏ಥ)", "(¬‿¬)", "(° ͜ʖ °)", "(✧ω✧)", "(ಠ‿ಠ)", "(͡° ͜ʖ ͡°)", "(¬‿¬)",
	"(ノಠ益ಠ)ノ彡┻━┻", "ʕ•ᴥ•ʔ", "(ง •̀_•́)ง",
	"(づ｡◕‿‿◕｡)づ", "(づ￣ ³￣)づ", "¯\\_(ツ)_/¯", "(☞ﾟヮﾟ)☞", "(╥﹏╥)", "(¬‿¬)",
	"ᕕ( ᐛ )ᕗ", "(╯︵╰,)", "(✿◕‿◕)", "ლ(ಠ益ಠლ)", "(>^.^<)", "(♥_♥)", "(ಠ⌣ಠ)",
	"(ʘ‿ʘ)", "(ʘ‿ʘ)ノ✿", "(╬ಠ益ಠ)", "(ง'̀-'́)ง", "(✖╭╮✖)", "(ಥ‿ಥ)", "(⊙_☉)",
	"(☉_☉)", "(╯_╰)", "( ͡ᵔ ͜ʖ ͡ᵔ )", "(ᵔᴥᵔ)", "(≧◡≦)", "(ﾉ◕ヮ◕)ﾉ*:・ﾟ✧", "(ಠ‿↼)",
	"(✪ω✪)", "(∩｀-´)⊃━☆ﾟ.*･｡ﾟ", "(づ￣ ³￣)づ💖", "┌( ಠ_ಠ)┘", "(╭ರᴥ•́)",
	"(❛‿❛)", "(⊙_◎)", "（〜^∇^)〜", "ᕦ(ò_óˇ)ᕤ", "⊂(◉‿◉)つ", "(╯°□°）╯︵ ( .o.)",
	"(¬‿¬)", "ಠ╭╮ಠ", "༼ つ ◕_◕ ༽つ", "(╯⊙ ⊱ ⊙╰ )", "( ಠ益ಠ )", "ಥ_ಥ",
	"( ͡° ͜ʖ ͡°)", "(☞ﾟヮﾟ)☞ ʕ•ᴥ•ʔ", "(ノಥ,_｣ಥ)ノ", "(ᗒᗣᗕ)՞", "୧༼ಠ益ಠ༽୨",
	"(☯‿☯✿)", "(✧Д✧)", "(ʘᴗʘ✿)", "(つ▀¯▀)つ", "(ง'̀-'́)ง", "(⚆_⚆)",
	"ಥ益ಥ", "(°ヘ°)", "(⊙﹏⊙)", "(⊃｡•́‿•̀｡)⊃",
}

func TotalRandomEmoji() string {
	if rand.Intn(2) == 1 {
		return RandomEmoticonUTF()
	}
	return fmt.Sprintf("%s %s %s", RandomEmoji(), RandomEmoji(), RandomEmoji())
}

func RandomEmoticonUTF() string {
	return _emoticonsUTF[rand.Intn(len(_emoticonsUTF))]
}

func RandomEmoji() string {
	// http://apps.timwhitlock.info/emoji/tables/unicode
	emoji := [][]int{
		// Emoticons icons
		{128513, 128591},
		// Transport and map symbols
		{128640, 128704},
	}
	r := emoji[rand.Int()%len(emoji)]
	min := r[0]
	max := r[1]
	n := rand.Intn(max-min+1) + min
	return html.UnescapeString("&#" + strconv.Itoa(n) + ";")
}

func TgText(text string) string {
	return bot.EscapeMarkdownUnescaped(text)
}

func TgLink(description, link string) string {
	return fmt.Sprintf("[%s](%s)", bot.EscapeMarkdownUnescaped(description), bot.EscapeMarkdownUnescaped(link))
}

func TimeToRu(t time.Time) string {
	return t.Format("15:04 02.01.2006") + fmt.Sprintf(" (%s)", GetTimeZone())
}

func TimeToRuWithSeconds(t time.Time) string {
	return t.Format("15:04:05 02.01.2006") + fmt.Sprintf(" (%s)", GetTimeZone())
}

func GetTimeZone() string {
	zone, _ := time.Now().Zone()
	return zone
}

func EscapeMarkdownV2(input string) string {
	// Регулярное выражение для поиска всех специальных символов MarkdownV2
	re := regexp.MustCompile(`([_*\[\]()~>#+\-=\|{}.!\\])`) // Добавлен символ `\` для экранирования
	return re.ReplaceAllString(input, `\$1`)                // Экранируем найденные символы
}

func SliceByRunes(s string, start, end int) string {
	// Преобразуем строку в срез рун
	runes := []rune(s)

	// Проверяем, что start и end не выходят за пределы длины среза рун
	if start < 0 || start > len(runes) || end > len(runes) || start > end {
		// Если end выходит за пределы, возвращаем исходную строку
		return s
	}

	// Возвращаем срез строк, преобразованный обратно в строку
	return string(runes[start:end])
}

func SliceToLastDot(s string) string {
	// Находим индекс последней точки
	index := strings.LastIndex(s, ".")

	// Если точка найдена, обрезаем строку до этого индекса (включая точку)
	if index != -1 {
		return s[:index+1]
	}

	// Если точки нет, возвращаем строку целиком
	return s
}

func TrimToFirstNewline(s string) string {
	// Находим индекс первого символа переноса строки
	index := strings.Index(s, "\n")

	// Если перенос строки найден, обрезаем строку до этого индекса
	if index != -1 {
		return s[:index]
	}
	// Если перенос строки не найден, возвращаем строку целиком
	return s
}

func RemoveExtraNewlines(input string) string {
	// Используем регулярное выражение, чтобы заменить последовательности \n на один \n
	re := regexp.MustCompile(`\n+`)
	return re.ReplaceAllString(input, "\n")
}
