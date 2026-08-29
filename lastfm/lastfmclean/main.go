package lastfmclean

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultMaxLength is used when no length is given to NewCleaner.
const DefaultMaxLength = 500

// Cleaner handles bio cleaning.
type Cleaner struct {
	maxLength int
}

// NewCleaner creates a new cleaner instance.
func NewCleaner(maxLength ...int) *Cleaner {
	ml := DefaultMaxLength
	if len(maxLength) > 0 && maxLength[0] > 0 {
		ml = maxLength[0]
	}
	return &Cleaner{maxLength: ml}
}

var (
	htmlRegex         = regexp.MustCompile(`<[^>]*>`)
	markdownLinkRegex = regexp.MustCompile(`\[(.*?)\]\(.*?\)`)
	readMoreRegex     = regexp.MustCompile(`(?i)read\s+more\s+on\s+last\.?fm`)
	referencesRegex   = regexp.MustCompile(`\[\d+]`)
	whitespaceRegex   = regexp.MustCompile(`\s+`)

	// Dedicated URL regex to protect domain dots (http://, https://, ftp://, or www.)
	urlRegex = regexp.MustCompile(`(?i)(?:https?://|ftp://|www\.)[^\s<>"{}|\^~\[\]\\]+`)

	// Match common language/unit/text abbreviations to preserve internal/trailing periods
	abbreviationRegex = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])((?:англ|рус|фр|нем|ит|исп|порт|кит|яп|кор|араб|греч|лат|сокр|ред|т\.е|т\.д|т\.п|н\.э|до\s*н\.э|пр|др|см|напр|и\s*т\.д|и\s*т\.п|т\.к|т\.н|стр|гл|св|сл|обл|ул|пер|бульв|просп|шосс|наб|пл|корп|лит|эт|подъезд|кв|комн|дом|д|стр|пом|оф|тел|факс|моб|email|e-mail|www|http|https|ftp|руб|USD|EUR|GBP|JPY|CAD|AUD|CHF|CNY|кг|г|мг|км|м|см|мм|л|мл|га|ac|dc|bc|ad|ce|bce|am|pm|vs|etc|eg|ie|cf|ca|approx|est|min|max|avg|std|dev|var|temp|pres|vol|no|nr|fig|p|pp|ch|sec|yr|mo|wk|day|hr|min|sec|deg|rad|mol|cd|Hz|dB|W|V|A|Ω|F|H|T|Wb|lm|lx|Bq|Gy|Sv|kat|m/s|m/s²|kg/m³|N/m²|Pa|J|N·m|W·s|C·V|V·A|W/A|A·s|V/A|Ω·m|S/m|Wb/A|H/m|J/K|J/(kg·K)|J/mol|J/mol·K|C/kg|Gy/s|W/sr)\.)`)
)

var (
	unicodeEscapeReplacer = strings.NewReplacer(
		`\\u003C`, "<",
		`\\u003E`, ">",
		`\\u0026`, "&",
		`\u003C`, "<",
		`\u003E`, ">",
		`\u0026`, "&",
	)
	escapeReplacer = strings.NewReplacer(
		`\\`, "",
		`\n`, " ",
		`\t`, " ",
		`\"`, `"`,
		`\r`, "",
	)
	multipleArtistPhrasePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)There are \d+ artists with this name`),
		regexp.MustCompile(`(?i)There are multiple artists under the name of`),
		regexp.MustCompile(`(?i)Multiple artists share this name`),
		regexp.MustCompile(`(?i)Artists sharing this name`),
		regexp.MustCompile(`(?i)There are at least \d+ other known artists? called`),
		regexp.MustCompile(`(?i)There are at least \d+ other artists? (?:with this name|called)`),
		regexp.MustCompile(`(?i)There (?:are|is) (?:at least )?\d+ (?:other )?(?:known )?artists? (?:with this name|called|named)`),
		regexp.MustCompile(`(?i)There is more than one artist that goes by the name`),
		regexp.MustCompile(`(?i)There (?:is|are) more than one artist`),
		regexp.MustCompile(`(?i)There are multiple artists that have performed under the name`),
	}
	listItemRegex  = regexp.MustCompile(`(?:^|\s)(?:[1-9]|[1-9][0-9])[\)\.]\s+`)
	headerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^There are \d+ artists with this name[:\s]*`),
		regexp.MustCompile(`(?i)^There are multiple artists under the name of [^:\n]+[:\s]*`),
		regexp.MustCompile(`(?i)^Multiple artists share this name[:\s]*`),
		regexp.MustCompile(`(?i)^Artists sharing this name[:\s]*`),
		regexp.MustCompile(`(?i)^There are at least \d+ other known artists? called [^:\n]*[:\s\-]*`),
		regexp.MustCompile(`(?i)^There (?:are|is) (?:at least )?\d+ (?:other )?(?:known )?artists? (?:with this name|called|named)[^:\n]*[:\s\-]*`),
		regexp.MustCompile(`(?i)^There is more than one artist that goes by the name [^:\n]*[:\s]*`),
		regexp.MustCompile(`(?i)^There (?:is|are) more than one artist[^:\n]*[:\s]*`),
		regexp.MustCompile(`(?i)^There are multiple artists that have performed under the name [^:\n]+[:\s]*`),
		regexp.MustCompile(`^\s*\d+[\)\.]\s*`),
	}
	bareNumberMarkerRegex = regexp.MustCompile(`^[1-9]\s+([A-Z])`)
	sectionEndPatterns    = []*regexp.Regexp{
		regexp.MustCompile(`\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`(?:^|[^\d])\s*\d+[\)\.]\s+[A-Z]`),
		regexp.MustCompile(`-{2,}\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n[A-Z][^\n]{0,50}(?:\n|$)`),
	}
	pairedChars = map[rune]rune{
		'(': ')', '[': ']', '{': '}', '«': '»', '‹': '›', '“': '”',
	}
	closingChars = map[rune]struct{}{
		')': {}, ']': {}, '}': {}, '»': {}, '›': {}, '”': {},
	}
)

// Clean performs sanitization, structural extractions, and smart truncation.
func (c *Cleaner) Clean(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	text = htmlRegex.ReplaceAllString(text, "")
	text = markdownLinkRegex.ReplaceAllString(text, "$1")
	text = referencesRegex.ReplaceAllString(text, "")

	if loc := readMoreRegex.FindStringIndex(text); loc != nil {
		text = text[:loc[0]]
	}

	text = unicodeEscapeReplacer.Replace(text)
	text = escapeReplacer.Replace(text)

	text = extractFirstArtistSection(text)
	text = strings.TrimSpace(whitespaceRegex.ReplaceAllString(text, " "))
	text = smartTruncate(text, c.maxLength)
	return strings.TrimSpace(text)
}

func extractFirstArtistSection(s string) string {
	s = strings.TrimSpace(s)
	matches := listItemRegex.FindAllStringIndex(s, -1)

	if len(matches) >= 2 {
		firstStart := matches[0][0]
		secondStart := matches[1][0]
		s = strings.TrimSpace(s[firstStart:secondStart])
	} else if hasExplicitMultipleArtists(s) {
		cutAt := -1
		for _, p := range multipleArtistPhrasePatterns {
			if loc := p.FindStringIndex(s); loc != nil && loc[0] > 0 {
				if cutAt < 0 || loc[0] < cutAt {
					cutAt = loc[0]
				}
			}
		}
		if cutAt > 0 {
			s = strings.TrimSpace(s[:cutAt])
		} else if len(matches) >= 1 {
			s = strings.TrimSpace(s[matches[0][0]:])
		} else {
			for _, p := range sectionEndPatterns {
				if loc := p.FindStringIndex(s); loc != nil {
					s = strings.TrimSpace(s[:loc[0]])
					break
				}
			}
		}
	}

	for _, p := range headerPatterns {
		s = p.ReplaceAllString(s, "")
	}
	s = bareNumberMarkerRegex.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

func hasExplicitMultipleArtists(s string) bool {
	for _, p := range multipleArtistPhrasePatterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

func smartTruncate(s string, limit int) string {
	sentences := splitSentences(s)
	if len(sentences) == 0 {
		if limit > 0 && utf8.RuneCountInString(s) > limit {
			return hardTruncate(s, limit)
		}
		return s
	}

	lastIdx := len(sentences) - 1
	if lastIdx > 0 {
		lastSent := strings.TrimSpace(sentences[lastIdx])
		if lastSent != "" {
			lastRune, _ := utf8.DecodeLastRuneInString(lastSent)
			if !isSentenceEnd(lastRune) {
				sentences = sentences[:lastIdx]
			}
		}
	}

	underLimit := limit <= 0 || utf8.RuneCountInString(s) <= limit
	var b strings.Builder
	length := 0

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}
		sentLen := utf8.RuneCountInString(sent)
		sep := 0
		if b.Len() > 0 {
			sep = 1
		}
		if !underLimit && length+sep+sentLen > limit {
			break
		}
		if sep == 1 {
			b.WriteString(" ")
		}
		b.WriteString(sent)
		length += sep + sentLen
	}

	if out := strings.TrimSpace(b.String()); out != "" {
		return balancePairedCharacters(out)
	}
	if underLimit {
		return s
	}
	return hardTruncate(s, limit)
}

func splitSentences(s string) []string {
	// Step 1: Mask dots in full URLs
	protected := urlRegex.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ReplaceAll(m, ".", "§URLDOT§")
	})

	// Step 2: Mask dots in known abbreviations
	protected = abbreviationRegex.ReplaceAllStringFunc(protected, func(m string) string {
		return strings.ReplaceAll(m, ".", "§ABBR§")
	})

	runes := []rune(protected)
	var sentences []string
	var current strings.Builder
	var stack []rune

	unmask := func(str string) string {
		str = strings.ReplaceAll(str, "§URLDOT§", ".")
		return strings.ReplaceAll(str, "§ABBR§", ".")
	}

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		current.WriteRune(r)

		if _, ok := pairedChars[r]; ok {
			stack = append(stack, r)
			continue
		}
		if _, ok := closingChars[r]; ok && len(stack) > 0 {
			if pairedChars[stack[len(stack)-1]] == r {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if isSentenceEnd(r) && len(stack) == 0 {
			for i+1 < len(runes) && isSentenceEnd(runes[i+1]) {
				i++
				current.WriteRune(runes[i])
			}
			sentences = append(sentences, strings.TrimSpace(unmask(current.String())))
			current.Reset()
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(unmask(current.String())))
	}
	return sentences
}

func balancePairedCharacters(s string) string {
	var stack []rune
	var b strings.Builder
	for _, r := range s {
		if _, ok := pairedChars[r]; ok {
			stack = append(stack, r)
			b.WriteRune(r)
		} else if _, ok := closingChars[r]; ok {
			if len(stack) > 0 && pairedChars[stack[len(stack)-1]] == r {
				stack = stack[:len(stack)-1]
				b.WriteRune(r)
			}
		} else {
			b.WriteRune(r)
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		b.WriteRune(pairedChars[stack[i]])
	}
	return b.String()
}

func hardTruncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	const ellipsis = "..."
	ellLen := utf8.RuneCountInString(ellipsis)
	if limit <= ellLen {
		return string(runes[:limit])
	}

	maxContent := limit - ellLen
	for contentLimit := maxContent; contentLimit > maxContent/2; contentLimit-- {
		for i := contentLimit; i > contentLimit/2; i-- {
			if i >= len(runes) {
				continue
			}
			if unicode.IsSpace(runes[i]) || unicode.IsPunct(runes[i]) {
				cutStr := string(runes[:i])
				cut := strings.TrimRightFunc(cutStr, func(r rune) bool {
					return unicode.IsPunct(r) || unicode.IsSpace(r)
				})
				result := balancePairedCharacters(cut + ellipsis)
				if utf8.RuneCountInString(result) <= limit {
					return result
				}
			}
		}
	}

	result := balancePairedCharacters(string(runes[:maxContent]) + ellipsis)
	r := []rune(result)
	if len(r) > limit {
		return string(r[:limit])
	}
	return result
}

func isSentenceEnd(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '…'
}
