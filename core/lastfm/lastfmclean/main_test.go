package lastfmclean

import (
	"testing"
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
			Expected: "Блэйн Мьюз (англ. Blane Muise; 4 мая 1993 года, Лондон, Англия), более известна под своим сценическим псевдонимом Shygirl — британский рэпер, диджей...",
			Length:   150,
		},
		{
			Input:    "Few artists with this bio: \n1) Artist Name (род. 1990) — американский [музыкант](http://example.com). \n\n2) Another artist description.",
			Expected: "Artist Name (род. 1990) — американский музыкант.",
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
