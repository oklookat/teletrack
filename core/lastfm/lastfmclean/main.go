// Written by: Claude (claude.ai), Grok (grok.com)
package lastfmclean

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultMaxLength is used when no length is given to NewCleaner.
const DefaultMaxLength = 500

// Cleaner handles bio cleaning. The only thing that varies between
// instances is the output length cap — every other cleaning step
// (HTML stripping, markdown links, references, "read more", first-section
// extraction) always runs.
type Cleaner struct {
	maxLength int
}

// NewCleaner creates a new cleaner. Pass a length to override the default,
// e.g. NewCleaner(300). NewCleaner() uses DefaultMaxLength.
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
	abbreviationRegex = regexp.MustCompile(`\b(англ|рус|фр|нем|ит|исп|порт|кит|яп|кор|араб|греч|лат|сокр|ред|т\.е|т\.д|т\.п|н\.э|до\s*н\.э|пр|др|см|напр|и\s*т\.д|и\s*т\.п|т\.к|т\.н|стр|гл|св|сл|обл|ул|пер|бульв|просп|шосс|наб|пл|корп|лит|эт|подъезд|кв|комн|дом|д|стр|пом|оф|тел|факс|моб|email|e-mail|www|http|https|ftp|руб|USD|EUR|GBP|JPY|CAD|AUD|CHF|CNY|кг|г|мг|км|м|см|мм|л|мл|га|ac|dc|bc|ad|ce|bce|am|pm|vs|etc|eg|ie|cf|ca|approx|est|min|max|avg|std|dev|var|temp|pres|vol|no|nr|fig|p|pp|ch|sec|yr|mo|wk|day|hr|min|sec|deg|rad|mol|cd|Hz|dB|W|V|A|Ω|F|H|T|Wb|lm|lx|Bq|Gy|Sv|kat|m/s|m/s²|kg/m³|N/m²|Pa|J|N·m|W·s|C·V|V·A|W/A|A·s|V/A|Ω·m|S/m|Wb/A|H/m|J/K|J/(kg·K)|J/mol|J/mol·K|C/kg|Gy/s|W/sr)\.`)
)

var (
	// unicodeEscapeReplacer decodes single- and double-escaped \u003C-style
	// sequences that sometimes leak in from JSON payloads.
	unicodeEscapeReplacer = strings.NewReplacer(
		`\\u003C`, "<",
		`\\u003E`, ">",
		`\\u0026`, "&",
		`\u003C`, "<",
		`\u003E`, ">",
		`\u0026`, "&",
	)
	// escapeReplacer decodes literal \n, \t, \", \r, \\ escape sequences.
	// Real newline/tab bytes are untouched here on purpose — they're still
	// needed by extractFirstArtistSection further down the pipeline.
	escapeReplacer = strings.NewReplacer(
		`\\`, "",
		`\n`, " ",
		`\t`, " ",
		`\"`, `"`,
		`\r`, "",
	)
	// Phrases that explicitly say the bio covers multiple artists.
	multipleArtistPhrasePatterns = []*regexp.Regexp{
		regexp.MustCompile(`There are \d+ artists with this name`),
		regexp.MustCompile(`There are multiple artists under the name of`),
		regexp.MustCompile(`Multiple artists share this name`),
		regexp.MustCompile(`Artists sharing this name`),
	}
	// listItemRegex matches enumeration markers like "1) " or "2. ",
	// whether separated by newlines or just spaces.
	listItemRegex = regexp.MustCompile(`(?:^|\s)([1-9]|[1-9][0-9])[\)\.]\s+`)
	// Header text to strip once we know we're dealing with the first
	// section of a multi-artist bio.
	headerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^There are \d+ artists with this name[:\s]*`),
		regexp.MustCompile(`(?i)^There are multiple artists under the name of [^:\n]+[:\s]*`),
		regexp.MustCompile(`(?i)^Multiple artists share this name[:\s]*`),
		regexp.MustCompile(`(?i)^Artists sharing this name[:\s]*`),
		regexp.MustCompile(`^\s*\d+[\)\.]\s*`),
	}
	// Where the first artist's section ends and the next one begins.
	sectionEndPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n[A-Z][^\n]{0,50}\n`),
	}
	// Paired characters that must stay balanced after truncation.
	pairedChars = map[rune]rune{
		'(': ')', '[': ']', '{': '}', '«': '»', '‹': '›', '“': '”',
	}
	closingChars = map[rune]struct{}{
		')': {}, ']': {}, '}': {}, '»': {}, '›': {}, '”': {},
	}
)

// Clean cleans a bio...
func (c *Cleaner) Clean(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	// Strip real HTML/markdown/reference markup first...
	text = htmlRegex.ReplaceAllString(text, "")
	text = markdownLinkRegex.ReplaceAllString(text, "$1")
	text = referencesRegex.ReplaceAllString(text, "")

	// "Read more on Last.fm" marks the end of the usable bio
	if loc := readMoreRegex.FindStringIndex(text); loc != nil {
		text = text[:loc[0]]
	}

	text = unicodeEscapeReplacer.Replace(text)
	text = escapeReplacer.Replace(text)

	// Must run while real newlines are still present
	text = extractFirstArtistSection(text)
	text = strings.TrimSpace(whitespaceRegex.ReplaceAllString(text, " "))
	text = smartTruncate(text, c.maxLength)
	return strings.TrimSpace(text)
}

func extractFirstArtistSection(s string) string {
	s = strings.TrimSpace(s)
	switch matches := listItemRegex.FindAllStringIndex(s, -1); {
	case len(matches) >= 2:
		// Keep only up to the second marker and strip intro text before first
		if len(matches) > 0 {
			firstStart := matches[0][0]
			secondStart := matches[1][0]
			s = strings.TrimSpace(s[firstStart:secondStart])
		}
	case hasExplicitMultipleArtists(s):
		for _, p := range sectionEndPatterns {
			if loc := p.FindStringIndex(s); loc != nil {
				s = strings.TrimSpace(s[:loc[0]])
				break
			}
		}
	}

	for _, p := range headerPatterns {
		s = p.ReplaceAllString(s, "")
	}
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

// smartTruncate, splitSentences, balancePairedCharacters и hardTruncate остались почти без изменений (кроме улучшения в hardTruncate)

func smartTruncate(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	sentences := splitSentences(s)
	var b strings.Builder
	length := 0
	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}
		first, _ := utf8.DecodeRuneInString(sent)
		if !unicode.IsUpper(first) && !unicode.IsDigit(first) {
			continue
		}
		sentLen := utf8.RuneCountInString(sent)
		if length+sentLen+1 > limit {
			truncated := balancePairedCharacters(b.String() + " " + sent)
			if utf8.RuneCountInString(truncated) <= limit {
				return truncated
			}
			break
		}
		if b.Len() > 0 {
			b.WriteString(" ")
			length++
		}
		b.WriteString(sent)
		length += sentLen
	}
	if out := strings.TrimSpace(b.String()); out != "" {
		return balancePairedCharacters(out)
	}
	return hardTruncate(s, limit)
}

func splitSentences(s string) []string {
	protected := abbreviationRegex.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ReplaceAll(m, ".", "§ABBR§")
	})
	var sentences []string
	var current strings.Builder
	var stack []rune
	for _, r := range protected {
		current.WriteRune(r)
		if _, ok := pairedChars[r]; ok {
			stack = append(stack, r)
			continue
		} else if _, ok := closingChars[r]; ok && len(stack) > 0 {
			if pairedChars[stack[len(stack)-1]] == r {
				stack = stack[:len(stack)-1]
			}
		}
		if isSentenceEnd(r) && len(stack) == 0 {
			sentence := strings.ReplaceAll(current.String(), "§ABBR§", ".")
			sentences = append(sentences, strings.TrimSpace(sentence))
			current.Reset()
		}
	}
	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(strings.ReplaceAll(current.String(), "§ABBR§", ".")))
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
	for _, s := range slices.Backward(stack) {
		b.WriteRune(pairedChars[s])
	}
	return b.String()
}

func hardTruncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	for i := limit - 1; i > limit/2; i-- {
		if unicode.IsSpace(runes[i]) || unicode.IsPunct(runes[i]) {
			cutStr := string(runes[:i])
			// Trim trailing punctuation and spaces to avoid dangling commas etc.
			cut := strings.TrimRightFunc(cutStr, func(r rune) bool {
				return unicode.IsPunct(r) || unicode.IsSpace(r)
			})
			return balancePairedCharacters(cut + "...")
		}
	}
	return balancePairedCharacters(string(runes[:limit]) + "...")
}

func isSentenceEnd(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '…'
}
