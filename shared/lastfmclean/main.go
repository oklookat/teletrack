package lastfmclean

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Config holds configuration for bio cleaning
type Config struct {
	MaxLength        int
	RemoveHTML       bool
	RemoveReferences bool
	RemoveReadMore   bool
	ExtractFirstOnly bool
	RemoveMarkdown   bool
}

// DefaultConfig returns default cleaning settings
func DefaultConfig() Config {
	return Config{
		MaxLength:        500,
		RemoveHTML:       true,
		RemoveReferences: true,
		RemoveReadMore:   true,
		ExtractFirstOnly: true,
		RemoveMarkdown:   true,
	}
}

// Cleaner handles bio cleaning
type Cleaner struct {
	config            Config
	htmlRegex         *regexp.Regexp
	markdownLinkRegex *regexp.Regexp
	readMoreRegex     *regexp.Regexp
	referencesRegex   *regexp.Regexp
	whitespaceRegex   *regexp.Regexp
	abbreviationRegex *regexp.Regexp
}

var (
	// Patterns for multiple artists
	multipleArtistPatterns = []*regexp.Regexp{
		regexp.MustCompile(`There are \d+ artists with this name`),
		regexp.MustCompile(`There are multiple artists under the name of`),
		regexp.MustCompile(`Multiple artists share this name`),
		regexp.MustCompile(`Artists sharing this name`),
		regexp.MustCompile(`\n\s*\d+[\)\.]\s`),
	}

	// Patterns for headers to remove
	headerPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^There are \d+ artists with this name[:\s]*`),
		regexp.MustCompile(`(?i)^There are multiple artists under the name of [^:\n]+[:\s]*`),
		regexp.MustCompile(`(?i)^Multiple artists share this name[:\s]*`),
		regexp.MustCompile(`(?i)^Artists sharing this name[:\s]*`),
		regexp.MustCompile(`^\s*\d+[\)\.]\s*`),
	}

	// End of section detection
	sectionEndPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n\s*\d+[\)\.]\s`),
		regexp.MustCompile(`\n\n[A-Z][^\n]{0,50}\n`),
	}

	// Paired characters
	pairedChars = map[rune]rune{
		'(': ')', '[': ']', '{': '}', '«': '»', '‹': '›', '“': '”',
	}

	closingChars = map[rune]struct{}{
		')': {}, ']': {}, '}': {}, '»': {}, '›': {}, '”': {},
	}
)

// NewCleaner creates a new cleaner
func NewCleaner(cfg ...Config) *Cleaner {
	c := DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return &Cleaner{
		config:            c,
		htmlRegex:         regexp.MustCompile(`<[^>]*>`),
		markdownLinkRegex: regexp.MustCompile(`\[(.*?)\]\(.*?\)`),
		readMoreRegex:     regexp.MustCompile(`(?i)read\s+more\s+on\s+last\.?fm`),
		referencesRegex:   regexp.MustCompile(`\[\d+]`),
		whitespaceRegex:   regexp.MustCompile(`\s+`),
		abbreviationRegex: regexp.MustCompile(`\b(англ|рус|фр|нем|ит|исп|порт|кит|яп|кор|араб|греч|лат|сокр|ред|т\.е|т\.д|т\.п|н\.э|до\s*н\.э|пр|др|см|напр|и\s*т\.д|и\s*т\.п|т\.к|т\.н|стр|гл|св|сл|обл|ул|пер|бульв|просп|шосс|наб|пл|корп|лит|эт|подъезд|кв|комн|дом|д|стр|пом|оф|тел|факс|моб|email|e-mail|www|http|https|ftp|руб|USD|EUR|GBP|JPY|CAD|AUD|CHF|CNY|кг|г|мг|км|м|см|мм|л|мл|га|ac|dc|bc|ad|ce|bce|am|pm|vs|etc|eg|ie|cf|ca|approx|est|min|max|avg|std|dev|var|temp|pres|vol|no|nr|fig|p|pp|ch|sec|yr|mo|wk|day|hr|min|sec|deg|rad|mol|cd|Hz|dB|W|V|A|Ω|F|H|T|Wb|lm|lx|Bq|Gy|Sv|kat|m/s|m/s²|kg/m³|N/m²|Pa|J|N·m|W·s|C·V|V·A|W/A|A·s|V/A|Ω·m|S/m|Wb/A|H/m|J/K|J/(kg·K)|J/mol|J/mol·K|C/kg|Gy/s|W/sr)\.`),
	}
}

// Clean cleans a bio according to configuration
func (c *Cleaner) Clean(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	text = c.decodeUnicodeEscapes(text)
	text = c.decodeEscapes(text)

	if c.config.RemoveHTML {
		text = c.htmlRegex.ReplaceAllString(text, "")
	}
	if c.config.RemoveMarkdown {
		text = c.markdownLinkRegex.ReplaceAllString(text, "$1")
	}
	if c.config.RemoveReferences {
		text = c.referencesRegex.ReplaceAllString(text, "")
	}
	if c.config.RemoveReadMore {
		text = c.readMoreRegex.ReplaceAllString(text, "")
	}

	text = strings.TrimSpace(c.whitespaceRegex.ReplaceAllString(strings.ReplaceAll(text, "\n", " "), " "))

	if c.config.ExtractFirstOnly {
		text = c.extractFirstArtistSection(text)
	}

	if c.config.MaxLength > 0 {
		text = c.smartTruncate(text, c.config.MaxLength)
	}

	return strings.TrimSpace(text)
}

// decodeUnicodeEscapes decodes common Unicode escapes
func (c *Cleaner) decodeUnicodeEscapes(s string) string {
	return strings.NewReplacer(
		`\\u003C`, "<",
		`\\u003E`, ">",
		`\\u0026`, "&",
		`\u003C`, "<",
		`\u003E`, ">",
		`\u0026`, "&",
	).Replace(s)
}

// decodeEscapes decodes \n, \t, \", etc
func (c *Cleaner) decodeEscapes(s string) string {
	replacer := strings.NewReplacer(
		`\\`, "",
		`\n`, " ",
		`\t`, " ",
		`\"`, `"`,
		`\r`, "",
	)
	s = replacer.Replace(s)
	return strings.TrimSpace(c.whitespaceRegex.ReplaceAllString(s, " "))
}

// extractFirstArtistSection extracts the first bio section
func (c *Cleaner) extractFirstArtistSection(s string) string {
	s = strings.TrimSpace(s)
	if c.hasMultipleArtists(s) {
		for _, p := range sectionEndPatterns {
			if loc := p.FindStringIndex(s); loc != nil {
				s = s[:loc[0]]
				break
			}
		}
	}

	for _, p := range headerPatterns {
		s = p.ReplaceAllString(s, "")
	}

	return strings.TrimSpace(s)
}

// hasMultipleArtists detects multiple artists in bio
func (c *Cleaner) hasMultipleArtists(s string) bool {
	for _, p := range multipleArtistPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// smartTruncate truncates text intelligently at sentence boundaries
func (c *Cleaner) smartTruncate(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}

	sentences := c.splitSentences(s)
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
			truncated := c.balancePairedCharacters(b.String() + " " + sent)
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
		return c.balancePairedCharacters(out)
	}
	return c.hardTruncate(s, limit)
}

// splitSentences splits text into sentences respecting abbreviations and paired chars
func (c *Cleaner) splitSentences(s string) []string {
	protected := c.abbreviationRegex.ReplaceAllStringFunc(s, func(m string) string {
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
			last := stack[len(stack)-1]
			if pairedChars[last] == r {
				stack = stack[:len(stack)-1]
			}
		}

		if c.isSentenceEnd(r) && len(stack) == 0 {
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

// balancePairedCharacters ensures all opened pairs are closed
func (c *Cleaner) balancePairedCharacters(s string) string {
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

// hardTruncate cuts text at word boundary and adds ellipsis
func (c *Cleaner) hardTruncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	for i := limit - 1; i > limit/2; i-- {
		if unicode.IsSpace(runes[i]) || unicode.IsPunct(runes[i]) {
			return c.balancePairedCharacters(string(runes[:i]) + "...")
		}
	}
	return c.balancePairedCharacters(string(runes[:limit]) + "...")
}

// isSentenceEnd checks sentence-ending punctuation
func (c *Cleaner) isSentenceEnd(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '…'
}
