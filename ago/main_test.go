package ago

import (
	"testing"
	"time"
)

func TestFormat(t *testing.T) {
	now := time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "just now",
			t:    now,
			want: "just now",
		},
		{
			name: "just now 12",
			t:    now.Add(-12 * time.Second),
			want: "12 sec ago",
		},
		{
			name: "just now 1",
			t:    now.Add(-1 * time.Second),
			want: "just now",
		},
		{
			name: "just now 59",
			t:    now.Add(-59 * time.Second),
			want: "59 sec ago",
		},
		{
			name: "1 minute ago",
			t:    now.Add(-1 * time.Minute),
			want: "1 min ago",
		},
		{
			name: "5 minutes ago",
			t:    now.Add(-5 * time.Minute),
			want: "5 min ago",
		},
		{
			name: "59 minutes ago",
			t:    now.Add(-59 * time.Minute),
			want: "59 min ago",
		},
		{
			name: "1 hour ago",
			t:    now.Add(-1 * time.Hour),
			want: "1 hour ago",
		},
		{
			name: "3 hours ago",
			t:    now.Add(-3 * time.Hour),
			want: "3 hours ago",
		},
		{
			name: "23 hours ago",
			t:    now.Add(-23 * time.Hour),
			want: "23 hours ago",
		},
		{
			name: "1 day ago",
			t:    now.Add(-24 * time.Hour),
			want: "1 day ago",
		},
		{
			name: "5 days ago",
			t:    now.Add(-5 * 24 * time.Hour),
			want: "5 days ago",
		},
		{
			name: "29 days ago",
			t:    now.Add(-29 * 24 * time.Hour),
			want: "29 days ago",
		},
		{
			name: "1 month ago",
			t:    now.Add(-30 * 24 * time.Hour),
			want: "1 month ago",
		},
		{
			name: "6 months ago",
			t:    now.Add(-180 * 24 * time.Hour),
			want: "6 months ago",
		},
		{
			name: "11 months ago",
			t:    now.Add(-330 * 24 * time.Hour),
			want: "11 months ago",
		},
		{
			name: "1 year ago",
			t:    now.Add(-365 * 24 * time.Hour),
			want: "1 year ago",
		},
		{
			name: "2 years ago",
			t:    now.Add(-2 * 365 * 24 * time.Hour),
			want: "2 years ago",
		},
		{
			name: "future",
			t:    now.Add(1 * time.Hour),
			want: "in the future",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format(tt.t, now)
			if got != tt.want {
				t.Errorf("format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormat_PublicAPI(t *testing.T) {
	got := Format(time.Now().Add(-12 * time.Second))
	if got == "" {
		t.Fatal("Format returned empty string")
	}
}
