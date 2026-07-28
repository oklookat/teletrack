package telegram

import (
	"fmt"
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
	tests := []struct {
		hour, min int
		want      string
	}{
		{0, 0, "🕛"},
		{0, 29, "🕛"},
		{0, 30, "🕧"},
		{0, 59, "🕧"},
		{1, 0, "🕐"},
		{1, 45, "🕜"},
		{2, 15, "🕑"},
		{2, 30, "🕝"},
		{3, 0, "🕒"},
		{4, 30, "🕟"},
		{5, 0, "🕔"},
		{6, 30, "🕡"},
		{7, 0, "🕖"},
		{8, 30, "🕣"},
		{9, 0, "🕘"},
		{10, 30, "🕥"},
		{11, 0, "🕚"},
		{11, 59, "🕦"},
		{12, 0, "🕛"},
		{12, 30, "🕧"},
		{13, 0, "🕐"},
		{15, 45, "🕞"},
		{23, 59, "🕦"},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%02d:%02d", tt.hour, tt.min)
		t.Run(name, func(t *testing.T) {
			tm := time.Date(2026, 7, 28, tt.hour, tt.min, 0, 0, time.UTC)
			got := clockEmoji(tm)
			if got != tt.want {
				t.Errorf("clockEmoji(%02d:%02d) = %q, want %q", tt.hour, tt.min, got, tt.want)
			}
		})
	}
}
