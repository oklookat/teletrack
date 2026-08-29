// Written by: Grok (grok.com)
package ago

import (
	"fmt"
	"time"
)

// Format returns a human-readable relative time string for t
// relative to the current moment (time.Now()).
func Format(t time.Time) string {
	return format(t, time.Now())
}

func format(t, now time.Time) string {
	if t.After(now) {
		return "in the future"
	}

	d := now.Sub(t)

	switch {
	case d < 5*time.Second:
		return "just now"
	case d < time.Minute:
		n := int(d.Seconds())
		return plural(n, "sec", "sec") // специально коротко
	case d < time.Hour:
		n := int(d.Minutes())
		return plural(n, "min", "min")
	case d < 24*time.Hour:
		n := int(d.Hours())
		return plural(n, "hour", "hours")
	case d < 30*24*time.Hour:
		n := int(d.Hours() / 24)
		return plural(n, "day", "days")
	case d < 365*24*time.Hour:
		n := int(d.Hours() / (24 * 30))
		if n < 1 {
			n = 1
		}
		return plural(n, "month", "months")
	default:
		n := int(d.Hours() / (24 * 365))
		if n < 1 {
			n = 1
		}
		return plural(n, "year", "years")
	}
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", singular)
	}
	return fmt.Sprintf("%d %s ago", n, pluralForm)
}
