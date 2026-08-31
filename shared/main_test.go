package shared

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPtr(t *testing.T) {
	t.Parallel()
	n := 42
	p := Ptr(n)
	if p == nil || *p != 42 {
		t.Fatalf("Ptr: got %#v", p)
	}
}

func TestGenerateTrackID(t *testing.T) {
	t.Parallel()
	if GenerateTrackID("", "x") != "" || GenerateTrackID("a", "") != "" {
		t.Fatal("empty inputs should yield empty ID")
	}
	a := GenerateTrackID("Artist", "Track")
	b := GenerateTrackID("Artist", "Track")
	c := GenerateTrackID("Artist", "Other")
	if a == "" || a != b {
		t.Fatalf("stable hash failed: %q %q", a, b)
	}
	if a == c {
		t.Fatal("different tracks must differ")
	}
}

func TestUnique(t *testing.T) {
	t.Parallel()
	got := Unique([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if Unique[int](nil) != nil {
		t.Fatal("nil in → nil out")
	}
}

func TestDurationUnmarshalJSON(t *testing.T) {
	t.Parallel()
	var d Duration
	if err := json.Unmarshal([]byte(`"1h30m"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Duration != 90*time.Minute {
		t.Fatalf("got %v", d.Duration)
	}
	if err := json.Unmarshal([]byte(`1000`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Duration != 1000 {
		t.Fatalf("got %v", d.Duration)
	}
}

func TestTrackProgressSupported(t *testing.T) {
	t.Parallel()
	p, d := 1, 10
	if !TrackProgressSupported(&p, &d) {
		t.Fatal("expected supported")
	}
	z := 0
	if TrackProgressSupported(&p, &z) || TrackProgressSupported(nil, &d) {
		t.Fatal("expected unsupported")
	}
}
