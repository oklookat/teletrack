package telegram

import (
	"testing"
	"time"
)

func TestTimeToRuWithSeconds(t *testing.T) {
	msk, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			name: "already moscow",
			in:   time.Date(2026, 7, 28, 15, 4, 5, 0, msk),
			want: "15:04:05 28.07.2026 (MSK)",
		},
		{
			name: "utc to moscow",
			in:   time.Date(2026, 7, 28, 12, 4, 5, 0, time.UTC),
			want: "15:04:05 28.07.2026 (MSK)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeToRuWithSeconds(tt.in)
			if got != tt.want {
				t.Errorf("timeToRuWithSeconds(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestClockEmoji(t *testing.T) {
	mk := func(h, m int) time.Time {
		return time.Date(2024, time.January, 1, h, m, 0, 0, time.UTC)
	}

	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		// Exact hour / half-hour marks.
		{"00:00 midnight -> 12:00", mk(0, 0), clockEmojis[0][0]},
		{"12:00 noon -> 12:00", mk(12, 0), clockEmojis[0][0]},
		{"01:00 -> 1:00", mk(1, 0), clockEmojis[1][0]},
		{"01:30 -> 1:30", mk(1, 30), clockEmojis[1][1]},
		{"13:00 -> 1:00 (PM wraps to 12h face)", mk(13, 0), clockEmojis[1][0]},
		{"23:30 -> 11:30", mk(23, 30), clockEmojis[11][1]},

		// Rounds down toward the current slot.
		{"13:14 rounds down -> 1:00", mk(13, 14), clockEmojis[1][0]},
		{"01:44 rounds down -> 1:30", mk(1, 44), clockEmojis[1][1]},
		{"11:44 rounds down -> 11:30", mk(11, 44), clockEmojis[11][1]},

		// Rounds up into the next half/hour (previously buggy: minute>=30 only
		// ever selected the "half" face of the *same* hour and could never
		// roll over into the next hour).
		{"13:45 rounds up -> 2:00", mk(13, 45), clockEmojis[2][0]},
		{"01:16 rounds up -> 1:30", mk(1, 16), clockEmojis[1][1]},
		{"11:50 rounds up -> 12:00", mk(11, 50), clockEmojis[0][0]},

		// Wraps across midnight / noon boundary.
		{"00:46 rounds up -> 1:00", mk(0, 46), clockEmojis[1][0]},
		{"23:46 rounds up -> 12:00 (wraps past midnight)", mk(23, 46), clockEmojis[0][0]},
		{"11:46 rounds up -> 12:00 (wraps past noon)", mk(11, 46), clockEmojis[0][0]},

		// Exact tie at :15/:45 rounds up (per the "+15, floor div 30" rule).
		{"01:15 tie rounds up -> 1:30", mk(1, 15), clockEmojis[1][1]},
		{"01:45 tie rounds up -> 2:00", mk(1, 45), clockEmojis[2][0]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clockEmoji(tt.in)
			if got != tt.want {
				t.Errorf(
					"clockEmoji(%02d:%02d) = %q, want %q",
					tt.in.Hour(), tt.in.Minute(), got, tt.want,
				)
			}
		})
	}
}

// TestClockEmojiAllMinutes is a sanity sweep ensuring every minute of every
// hour maps to a valid, non-empty emoji and that the function never panics
// (e.g. via out-of-range slice indexing after the modulo arithmetic).
func TestClockEmojiAllMinutes(t *testing.T) {
	for h := 0; h < 24; h++ {
		for m := 0; m < 60; m++ {
			tm := time.Date(2024, time.January, 1, h, m, 0, 0, time.UTC)
			got := clockEmoji(tm)
			if got == "" {
				t.Errorf("clockEmoji(%02d:%02d) returned empty string", h, m)
			}
		}
	}
}
