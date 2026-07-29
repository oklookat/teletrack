// Written by: Claude (claude.ai), Grok (grok.com)
package lastfmclean

import (
	"regexp"
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
	// Use a unicode-aware "word boundary" because Go's \b only considers ASCII [0-9A-Za-z_].
	// Match an optional non-letter/digit prefix so we can protect Cyrillic abbreviations like "т.е.".
	abbreviationRegex = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])((?:англ|рус|фр|нем|ит|исп|порт|кит|яп|кор|араб|греч|лат|сокр|ред|т\.е|т\.д|т\.п|н\.э|до\s*н\.э|пр|др|см|напр|и\s*т\.д|и\s*т\.п|т\.к|т\.н|стр|гл|св|сл|обл|ул|пер|бульв|просп|шосс|наб|пл|корп|лит|эт|подъезд|кв|комн|дом|д|стр|пом|оф|тел|факс|моб|email|e-mail|www|http|https|ftp|руб|USD|EUR|GBP|JPY|CAD|AUD|CHF|CNY|кг|г|мг|км|м|см|мм|л|мл|га|ac|dc|bc|ad|ce|bce|am|pm|vs|etc|eg|ie|cf|ca|approx|est|min|max|avg|std|dev|var|temp|pres|vol|no|nr|fig|p|pp|ch|sec|yr|mo|wk|day|hr|min|sec|deg|rad|mol|cd|Hz|dB|W|V|A|Ω|F|H|T|Wb|lm|lx|Bq|Gy|Sv|kat|m/s|m/s²|kg/m³|N/m²|Pa|J|N·m|W·s|C·V|V·A|W/A|A·s|V/A|Ω·m|S/m|Wb/A|H/m|J/K|J/(kg·K)|J/mol|J/mol·K|C/kg|Gy/s|W/sr)\.)`)
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
		regexp.MustCompile(`(?i)There are \d+ artists with this name`),
		regexp.MustCompile(`(?i)There are multiple artists under the name of`),
		regexp.MustCompile(`(?i)Multiple artists share this name`),
		regexp.MustCompile(`(?i)Artists sharing this name`),
		// Common Last.fm variant: "There are at least N other known artists called X"
		regexp.MustCompile(`(?i)There are at least \d+ other known artists? called`),
		regexp.MustCompile(`(?i)There are at least \d+ other artists? (?:with this name|called)`),
		regexp.MustCompile(`(?i)There (?:are|is) (?:at least )?\d+ (?:other )?(?:known )?artists? (?:with this name|called|named)`),
		regexp.MustCompile(`(?i)There is more than one artist that goes by the name`),
		regexp.MustCompile(`(?i)There (?:is|are) more than one artist`),
		regexp.MustCompile(`(?i)There are multiple artists that have performed under the name`),
	}
	// listItemRegex matches enumeration markers like "1) " or "2. ",
	// whether separated by newlines or just spaces.
	listItemRegex = regexp.MustCompile(`(?:^|\s)(?:[1-9]|[1-9][0-9])[\)\.]\s+`)
	// Header text to strip once we know we're dealing with the first
	// section of a multi-artist bio.
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
	// bareNumberMarkerRegex strips a bare numbered marker with no punctuation
	// (e.g. "1 Artist Name ...") that can remain after a header phrase has
	// been stripped above. Restricted to a single digit immediately followed
	// by whitespace and then a capital letter, to avoid eating years/dates
	// like "1975 ...". Uses a capture group (instead of a lookahead, which
	// Go's RE2 engine doesn't support) so the following letter is preserved.
	bareNumberMarkerRegex = regexp.MustCompile(`^[1-9]\s+([A-Z])`)
	// Where the first artist's section ends and the next one begins.
	// Matches both newline-separated and inline "2. " / "2) " markers,
	// and also the common "------ 2." separator style.
	sectionEndPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n\s*\d+[\)\.]\s`),
		// Inline / same-line numbered section (no leading newline required)
		regexp.MustCompile(`(?:^|[^\d])\s*\d+[\)\.]\s+[A-Z]`),
		// Dashed separator followed by a numbered entry
		regexp.MustCompile(`-{2,}\s*\d+[\)\.]\s`),
		// Trailing newline is optional because TrimSpace may have removed it.
		regexp.MustCompile(`\n\n[A-Z][^\n]{0,50}(?:\n|$)`),
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
	matches := listItemRegex.FindAllStringIndex(s, -1)

	// 1. Two or more numbered entries → keep only the first section.
	if len(matches) >= 2 {
		firstStart := matches[0][0]
		secondStart := matches[1][0]
		s = strings.TrimSpace(s[firstStart:secondStart])
	} else if hasExplicitMultipleArtists(s) {
		// 2. Multi-artist phrase appears mid-bio → cut before it (primary bio first).
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
			// 3. Phrase is at the start and a numbered entry follows → start there.
			s = strings.TrimSpace(s[matches[0][0]:])
		} else {
			// 4. Fall back to section-end markers.
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
	// Always prefer complete sentences. Drop a trailing fragment that
	// does not end with terminal punctuation (typical of cut-off source
	// text). Only fall back to hard truncation when the length budget
	// forces us to. Artist names may start with a lowercase letter, so
	// we do NOT require an uppercase start.
	sentences := splitSentences(s)
	if len(sentences) == 0 {
		if limit > 0 && utf8.RuneCountInString(s) > limit {
			return hardTruncate(s, limit)
		}
		return s
	}

	// Drop a final incomplete fragment when earlier complete sentences exist.
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
	protected := abbreviationRegex.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ReplaceAll(m, ".", "§ABBR§")
	})
	runes := []rune(protected)
	var sentences []string
	var current strings.Builder
	var stack []rune
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
			// Collapse consecutive sentence terminators:
			// "..." / "?!" / "?.." — one boundary, not several.
			for i+1 < len(runes) && isSentenceEnd(runes[i+1]) {
				i++
				current.WriteRune(runes[i])
			}
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
	// Leave room for the ellipsis; balancePairedCharacters may add a few closers,
	// so we try progressively shorter cuts until the final result fits.
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
	// Fallback: hard cut content and append ellipsis, then force length.
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
