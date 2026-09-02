package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.json")
	s := LoadStateFrom(path)
	if len(s.LastUsed) != 0 {
		t.Fatalf("fresh state not empty: %+v", s)
	}
	s.Touch("CoolIfy")
	if err := s.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o", st.Mode().Perm())
	}
	got := LoadStateFrom(path)
	if got.Last("coolify").IsZero() || got.Last("COOLIFY") != got.Last("coolify") {
		t.Errorf("last_used lookup must be case-insensitive: %+v", got.LastUsed)
	}
	if !got.Last("other").IsZero() {
		t.Error("unknown host must be zero")
	}
	got.Forget("coolify")
	if !got.Last("coolify").IsZero() {
		t.Error("forget failed")
	}
}

func TestStateCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	os.WriteFile(path, []byte("{oops"), 0o600)
	s := LoadStateFrom(path)
	if s.LastUsed == nil {
		t.Error("corrupt state must yield empty map")
	}
}

func TestRelative(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	cases := map[time.Duration]string{
		0:                    "just now",
		30 * time.Second:     "just now",
		time.Minute:          "1 min ago",
		5 * time.Minute:      "5 mins ago",
		time.Hour:            "1 hour ago",
		3 * time.Hour:        "3 hours ago",
		26 * time.Hour:       "1 day ago",
		72 * time.Hour:       "3 days ago",
		45 * 24 * time.Hour:  "1 month ago",
		400 * 24 * time.Hour: "1 year ago",
	}
	for d, want := range cases {
		if got := Relative(now.Add(-d), now); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
	if got := Relative(time.Time{}, now); got != "never" {
		t.Errorf("zero: %q", got)
	}
}
