package lastfmclean

import (
	"testing"
)

func TestCleaner_Clean(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		input    string
		expected string
	}{
		{
			name: "Basic HTML and reference removal",
			config: Config{
				MaxLength:        500,
				RemoveHTML:       true,
				RemoveReferences: true,
				RemoveReadMore:   true,
				ExtractFirstOnly: false,
				RemoveMarkdown:   true,
			},
			input:    "Hello <b>world</b> with reference[1] and <a href=\"test\">link</a>",
			expected: "Hello world with reference and link",
		},
		{
			name: "Read more removal",
			config: Config{
				MaxLength:        500,
				RemoveHTML:       true,
				RemoveReferences: true,
				RemoveReadMore:   true,
				ExtractFirstOnly: false,
				RemoveMarkdown:   true,
			},
			input:    "Some bio text. Read more on Last.fm and continue",
			expected: "Some bio text. and continue",
		},
		{
			name: "Markdown removal",
			config: Config{
				MaxLength:        500,
				RemoveHTML:       true,
				RemoveReferences: true,
				RemoveReadMore:   true,
				ExtractFirstOnly: false,
				RemoveMarkdown:   true,
			},
			input:    "Check out [my website](http://example.com) for more info",
			expected: "Check out my website for more info",
		},
		{
			name: "Extract first artist section",
			config: Config{
				MaxLength:        500,
				RemoveHTML:       true,
				RemoveReferences: true,
				RemoveReadMore:   true,
				ExtractFirstOnly: true,
				RemoveMarkdown:   true,
			},
			input:    "1) First artist bio. 2) Second artist bio. 3) Third artist bio.",
			expected: "First artist bio.",
		},
		{
			name: "Unicode escapes decoding",
			config: Config{
				MaxLength:        500,
				RemoveHTML:       true,
				RemoveReferences: true,
				RemoveReadMore:   true,
				ExtractFirstOnly: false,
				RemoveMarkdown:   true,
			},
			input:    "Text with \\u003Chtml\\u003E tags \\u0026 symbols",
			expected: "Text with <html> tags & symbols",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaner := NewCleaner(tt.config)
			result := cleaner.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCleaner_SmartTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected string
	}{
		{
			name:     "Russian text with abbreviation",
			input:    "Блэйн Мьюз (англ. Blane Muise; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер, диджей, певица, автор песен, со-руководитель и со-основатель звукозаписывающей компании Nuxxe, прославившаяся своими расистскими высказыванием в адрес Slayyyter",
			limit:    100,
			expected: "Блэйн Мьюз (англ. Blane Muise; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaner := NewCleaner(Config{MaxLength: tt.limit})
			result := cleaner.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("SmartTruncate() = %q, want %q", result, tt.expected)
			}
			// Проверяем, что результат не превышает лимит (с учетом многоточия)
			if len(result) > tt.limit+3 && tt.limit > 10 {
				t.Errorf("Result length %d exceeds limit %d", len(result), tt.limit)
			}
		})
	}
}

func TestCleaner_SplitSentences(t *testing.T) {
	cleaner := NewCleaner(Config{})

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Simple sentences",
			input:    "First sentence. Second sentence! Third sentence?",
			expected: []string{"First sentence.", "Second sentence!", "Third sentence?"},
		},
		{
			name:     "Abbreviations",
			input:    "Dr. Smith arrived at 5 p.m. Then he left.",
			expected: []string{"Dr. Smith arrived at 5 p.m.", "Then he left."},
		},
		{
			name:     "Russian abbreviations",
			input:    "Блэйн Мьюз (англ. Blane Muise) родился в Лондоне. Он известный музыкант.",
			expected: []string{"Блэйн Мьюз (англ. Blane Muise) родился в Лондоне.", "Он известный музыкант."},
		},
		{
			name:     "Parentheses",
			input:    "This (with parenthetical content) is one sentence. This is another.",
			expected: []string{"This (with parenthetical content) is one sentence.", "This is another."},
		},
		{
			name:     "Quotes",
			input:    `She said "Hello world!" and smiled. Then she left.`,
			expected: []string{`She said "Hello world!" and smiled.`, "Then she left."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.splitSentences(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitSentences() returned %d sentences, want %d. Got: %v", len(result), len(tt.expected), result)
				return
			}
			for i, sent := range result {
				if sent != tt.expected[i] {
					t.Errorf("Sentence %d = %q, want %q", i, sent, tt.expected[i])
				}
			}
		})
	}
}

func TestCleaner_BalancePairedCharacters(t *testing.T) {
	cleaner := NewCleaner(Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unclosed parenthesis",
			input:    "Text (with unclosed parenthesis",
			expected: "Text (with unclosed parenthesis)",
		},
		{
			name:     "Unclosed brackets",
			input:    "Text [with nested (parentheses and unclosed brackets",
			expected: "Text [with nested (parentheses and unclosed brackets)]",
		},
		{
			name:     "Extra closing",
			input:    "Text ) with extra closing ) parentheses",
			expected: "Text with extra closing parentheses",
		},
		{
			name:     "Mixed quotes",
			input:    `She said "Hello world and smiled`,
			expected: `She said "Hello world and smiled"`,
		},
		{
			name:     "Already balanced",
			input:    "Text (with [properly {balanced}][ brackets])",
			expected: "Text (with [properly {balanced}][ brackets])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.balancePairedCharacters(tt.input)
			if result != tt.expected {
				t.Errorf("balancePairedCharacters() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCleaner_FullPipeline(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		limit    int
		expected string
	}{
		{
			name: "Slater bio",
			raw: `
1) Slater is a rapper from Southern California. He is a part of the Vada Vada collective along with founders The Garden, and members Enjoy, Cowgirl Clue, and Lumina, among others. Known best for tracks \"Trix\", featuring Enjoy and \"I'll Put It on Metal\".\n\n2) Former stagename of Slayyyter.\n\n3) Slater, also known as Head Honcho Slater, is a rapper from Long Beach, CA. He first garnered a small buzz on Tumblr when he released popular SoundCloud singles such as \"10 Toez 4 Tha Hoez\", \"Charles Bronson\", \"Let Me See It\", and \"You Already Know\". \u003Ca href=\"https://www.last.fm/music/Slater\"\u003ERead more on Last.fm\u003C/a\u003E
`,
			limit:    300,
			expected: `Slater is a rapper from Southern California. He is a part of the Vada Vada collective along with founders The Garden, and members Enjoy, Cowgirl Clue, and Lumina, among others. Known best for tracks "Trix", featuring Enjoy and "I'll Put It on Metal".`,
		},
		{
			name:  "Shygirl bio with abbreviation problem",
			raw:   "Блэйн Мьюз (англ. Blane Muise[1]; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер, диджей, певица, автор песен, со-руководитель и со-основатель звукозаписывающей компании Nuxxe, прославившаяся своими расистскими высказыванием в адрес Slayyyter <a href=\"https://www.last.fm/music/Shygirl\">Read more on Last.fm</a>",
			limit: 150,
			// Ожидаем, что не обрежет на "англ.", а найдет нормальное место для обрезки
			expected: "Блэйн Мьюз (англ. Blane Muise; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер, диджей...",
		},
		{
			name:     "Complex bio with multiple elements",
			raw:      "1) Artist Name (род. 1990) — американский [музыкант](http://example.com). Известен по работе с группой «The Band». Выпустил альбом в 2020 г. [2] Read more on Last.fm\n\n2) Another artist description.",
			limit:    200,
			expected: "Artist Name (род. 1990) — американский музыкант. Известен по работе с группой «The Band». Выпустил альбом в 2020 г.",
		},
		{
			name:     "Very short limit",
			raw:      "This is a very long sentence that should be truncated properly at word boundary.",
			limit:    20,
			expected: "This is a very...",
		},
		{
			name:     "Bio with technical terms",
			raw:      "Dr. John Smith (Ph.D. in Physics) works at approx. 100 m/s. He published papers in Nature etc. His research continues.",
			limit:    80,
			expected: "Dr. John Smith (Ph.D. in Physics) works at approx. 100 m/s. He published papers in Nature etc.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaner := NewCleaner(Config{
				MaxLength:        tt.limit,
				RemoveHTML:       true,
				RemoveReferences: true,
				RemoveReadMore:   true,
				ExtractFirstOnly: true,
				RemoveMarkdown:   true,
			})

			result := cleaner.Clean(tt.raw)
			if result != tt.expected {
				t.Errorf("CleanBio failed for %s\nGot:      %q\nExpected: %q\nLength: got=%d, expected=%d",
					tt.name, result, tt.expected, len(result), len(tt.expected))
			}

			// Дополнительная проверка: результат не должен превышать лимит (с допуском для многоточия)
			if len(result) > tt.limit+10 && tt.limit > 20 {
				t.Errorf("Result length %d exceeds limit %d for test %s", len(result), tt.limit, tt.name)
			}
		})
	}
}

// Тест для проверки обработки эскейп-последовательностей
func TestCleaner_DecodeEscapes(t *testing.T) {
	cleaner := NewCleaner(Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Newlines and tabs",
			input:    "Text with\\nnewlines\\tand tabs",
			expected: "Text with newlines and tabs",
		},
		{
			name:     "Quotes and slashes",
			input:    "Text with \\\"quotes\\\" and \\\\slashes",
			expected: `Text with "quotes" and slashes`,
		},
		{
			name:     "Mixed escapes",
			input:    "Line1\\nLine2\\tTab\\\"Quote\\\"",
			expected: "Line1 Line2 Tab\"Quote\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleaner.decodeEscapes(tt.input)
			if result != tt.expected {
				t.Errorf("decodeEscapes() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Бенчмарк-тест для проверки производительности
func BenchmarkCleaner_Clean(b *testing.B) {
	cleaner := NewCleaner(Config{
		MaxLength:        300,
		RemoveHTML:       true,
		RemoveReferences: true,
		RemoveReadMore:   true,
		ExtractFirstOnly: true,
		RemoveMarkdown:   true,
	})

	longBio := `
1) Блэйн Мьюз (англ. Blane Muise[1]; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер, диджей, певица, автор песен, со-руководитель и со-основатель звукозаписывающей компании Nuxxe. <a href="test">Read more on Last.fm</a>

2) Another artist description that should be removed during extraction.
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleaner.Clean(longBio)
	}
}
