package lastfmclean

import (
	"testing"
	"unicode/utf8"
)

type Case struct {
	Length   int
	Input    string
	Expected string
}

func TestCleaner_Clean(t *testing.T) {
	cases := []Case{
		{
			Input:    "Hello <b>world</b> with reference[1] and <a href=\"test\">link</a>",
			Expected: "Hello world with reference and link",
		},
		{
			Input:    "Some bio text. Read more on Last.fm and continue",
			Expected: "Some bio text.",
		},
		{
			Input:    "Check out [my website](http://example.com) for more info",
			Expected: "Check out my website for more info",
		},
		{
			Input:    "1) Hello. 2) World. 3) Qwerty.",
			Expected: "Hello.",
		},
		{
			Input:    "Text with\\nnewlines\\tand tabs",
			Expected: "Text with newlines and tabs",
		},
		{
			Input:    "Text with \\\"quotes\\\" and \\\\slashes",
			Expected: `Text with "quotes" and slashes`,
		},
		{
			Input:    "Line1\\nLine2\\tTab\\\"Quote\\\"",
			Expected: "Line1 Line2 Tab\"Quote\"",
		},
		{
			Input:    "Text with \\u003Chtml\\u003E tags \\u0026 symbols",
			Expected: "Text with <html> tags & symbols",
		},
		{
			Input:    "Блэйн Мьюз (англ. Blane Muise[1]; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер, диджей-:, певица, автор песен, со-руководитель и со-основатель звукозаписывающей компании Nuxxe, прославившаяся своими расистскими высказыванием в адрес Slayyyter <a href=\"https://www.last.fm/music/Shygirl\">Read more on Last.fm</a>",
			Expected: "Блэйн Мьюз (англ. Blane Muise; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер...",
			Length:   150,
		},
		{
			Input:    "Few artists with this bio: \n1) Artist Name (род. 1990) — американский [музыкант](http://example.com). \n\n2) Another artist description.",
			Expected: "Artist Name (род. 1990) — американский музыкант.",
		},
		{
			Input:    "Fetish or KUNTFETISH, whose real name is Kole Kimbro, is an Atlanta-based rapper, model, and exotic dancer. She has gained attention for her dynamic style and presence in the underground music scene. There are at least 8 other known artists called Fetish: ---------------------- 2. Fetish are three friends from Wolverhampton in the West Midlands, UK. They were active around 1983/4 and were Steve Cordery (Flash), Steve Chappell (Chap) and Nick Daker.",
			Expected: "Fetish or KUNTFETISH, whose real name is Kole Kimbro, is an Atlanta-based rapper, model, and exotic dancer. She has gained attention for her dynamic style and presence in the underground music scene.",
			Length:   1000,
		},
		{
			Input:    "meat computer is a rapper, producer, and digital artist from Manitoba, Canada, who’s known for his unique and experimental sound. He first began releasing music under the alias Earthboy in 2017 generally in a more shoegaze, bedroom-pop, and indie style. He would maintain this pitched-down style until early 2020, when he changed his name to meat computer and began rapping in an extremely high-pitched voice over his experimental production. meat",
			Expected: "meat computer is a rapper, producer, and digital artist from Manitoba, Canada, who’s known for his unique and experimental sound. He first began releasing music under the alias Earthboy in 2017 generally in a more shoegaze, bedroom-pop, and indie style. He would maintain this pitched-down style until early 2020, when he changed his name to meat computer and began rapping in an extremely high-pitched voice over his experimental production.",
		},
		{
			Input:    "There is more than one artist that goes by the name Hi-C: 1. Christian Shaw (born January 31, 1997), better known as Hi-C (aka yung hi c ^_^), is a rapper and producer from Nashville, Tennessee. He is also known as a former member of Diamondsonmydick's infamous rap collective Reptilian Club Boyz. Previous to the release of his debut album, No More Heroes, Vol.",
			Expected: "Christian Shaw (born January 31, 1997), better known as Hi-C (aka yung hi c ^_^), is a rapper and producer from Nashville, Tennessee. He is also known as a former member of Diamondsonmydick's infamous rap collective Reptilian Club Boyz.",
			Length:   240,
		},
	}

	cleaner := NewCleaner(500)

	for _, cased := range cases {
		if cased.Length < 1 {
			cleaner.maxLength = DefaultMaxLength
		} else {
			cleaner.maxLength = cased.Length
		}

		result := cleaner.Clean(cased.Input)
		if result != cased.Expected {
			t.Fatalf("\n\nexpected: %s\n\ngot: %s\n\ninput: %s", cased.Expected, result, cased.Input)
		}
	}
}

// --- Regression/end-to-end кейсы через Cleaner.Clean, дополняющие существующий TestCleaner_Clean ---

func TestCleaner_Clean_SentenceBoundaryRules(t *testing.T) {
	cases := []Case{
		{
			// "..." должно схлопнуться в одну границу предложения, а не в три.
			Input:    "This is a test... And another sentence here that is quite long to test truncation behavior further.",
			Expected: "This is a test...",
			Length:   20,
		},
		{
			// "?!" — тоже одна граница, оба знака должны попасть в первое предложение.
			Input:    "Are you kidding?! This is unbelievable and quite a long sentence to push past the limit for testing purposes today.",
			Expected: "Are you kidding?!",
			Length:   20,
		},
		{
			// Единственное предложение целиком не влезает — fallback на hardTruncate,
			// режем по последнему пробелу, без скобок балансировать нечего.
			Input:    "This is a very long sentence without an early stopping punctuation mark to break it up nicely for testing.",
			Expected: "This is a very long...",
			Length:   25,
		},
		{
			// То же самое, но с незакрытой скобкой — hardTruncate должен
			// сам закрыть скобку через balancePairedCharacters.
			Input:    "This (opens a parenthesis without closing properly for a very long time to test unbalanced truncation handling in our code.",
			Expected: "This (opens a...)",
			Length:   20,
		},
		{
			// Точка внутри сокращения не должна ломаться из-за нового
			// схлопывания повторяющихся терминаторов.
			Input:    "Псевдоним указан как т.е. основной артист, а не второстепенный. Второе предложение здесь идёт значительно длиннее для того чтобы точно не поместиться в лимит по символам совсем никак вообще.",
			Expected: "Псевдоним указан как т.е. основной артист, а не второстепенный.",
			Length:   70,
		},
	}

	cleaner := NewCleaner(500)
	for _, cased := range cases {
		if cased.Length < 1 {
			cleaner.maxLength = DefaultMaxLength
		} else {
			cleaner.maxLength = cased.Length
		}
		result := cleaner.Clean(cased.Input)
		if result != cased.Expected {
			t.Fatalf("\n\nexpected: %s\n\ngot: %s\n\ninput: %s", cased.Expected, result, cased.Input)
		}
	}
}

// --- Прямой тест на извлечение первой секции без нумерации, через явную фразу ---

func TestExtractFirstArtistSection_PhraseBasedNoNumbers(t *testing.T) {
	input := "There are multiple artists under the name of Test: First is a musician from Norway.\n\nSecond Artist is different.\n"
	expected := "First is a musician from Norway."

	got := extractFirstArtistSection(input)
	if got != expected {
		t.Fatalf("\n\nexpected: %s\n\ngot: %s", expected, got)
	}
}

// --- Юнит-тест на splitSentences: соседние терминаторы должны схлопываться ---

func TestSplitSentences_CollapsesRepeatedTerminators(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Wait... What just happened? Then it ended!!! Finally, silence.",
			expected: []string{"Wait...", "What just happened?", "Then it ended!!!", "Finally, silence."},
		},
		{
			input:    "Really?! Yes, really.",
			expected: []string{"Really?!", "Yes, really."},
		},
	}

	for _, tt := range tests {
		got := splitSentences(tt.input)
		if len(got) != len(tt.expected) {
			t.Fatalf("input %q: expected %d sentences %v, got %d %v", tt.input, len(tt.expected), tt.expected, len(got), got)
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Fatalf("input %q: sentence %d expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

// --- Юнит-тест на smartTruncate: должен брать только целые предложения,
// никогда не пытаясь впихнуть частично влезающее предложение ---

func TestSmartTruncate_OnlyWholeSentences(t *testing.T) {
	input := "First sentence here. Second sentence that is quite a bit longer than the first one. Third one."

	// Лимит достаточен для первого предложения, но не для первого+второго.
	got := smartTruncate(input, 25)
	expected := "First sentence here."
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// --- balancePairedCharacters: закрывает незакрытые скобки в конце ---

func TestBalancePairedCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"This (is unbalanced", "This (is unbalanced)"},
		{"Nested (parens (here", "Nested (parens (here))"},
		{"Already (balanced)", "Already (balanced)"},
		{"No parens at all", "No parens at all"},
	}
	for _, tt := range tests {
		if got := balancePairedCharacters(tt.input); got != tt.expected {
			t.Fatalf("input %q: expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

// --- Инвариант: результат никогда не должен превышать maxLength ---

func TestSmartTruncate_NeverExceedsLimit(t *testing.T) {
	inputs := []string{
		"Short.",
		"This is a test... And another sentence here that is quite long to test truncation behavior further.",
		"This (opens a parenthesis without closing properly for a very long time to test unbalanced truncation handling in our code.",
		"Одно предложение без знаков окончания вообще без точки в конце совсем никак нигде",
		"Sentence one. Sentence two! Sentence three? Sentence four... Sentence five.",
	}
	limits := []int{5, 10, 15, 20, 30, 50, 100}

	for _, in := range inputs {
		for _, limit := range limits {
			got := smartTruncate(in, limit)
			if n := utf8.RuneCountInString(got); n > limit {
				t.Fatalf("input %q limit %d: result %q has %d runes, exceeds limit", in, limit, got, n)
			}
		}
	}
}
