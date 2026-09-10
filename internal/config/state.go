package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type State struct {
	LastUsed map[string]time.Time `json:"last_used"`
	Setup    map[string]time.Time `json:"setup,omitempty"`
}

func StatePath() string {
	return filepath.Join(filepath.Dir(Path()), "state.json")
}

func LoadState() *State {
	return LoadStateFrom(StatePath())
}

func LoadStateFrom(path string) *State {
	s := &State{LastUsed: map[string]time.Time{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, s); err != nil || s.LastUsed == nil {
		s.LastUsed = map[string]time.Time{}
	}
	return s
}

func (s *State) SetupDone(name string) bool {
	_, ok := s.Setup[strings.ToLower(name)]
	return ok
}

func (s *State) MarkSetup(name string) {
	if s.Setup == nil {
		s.Setup = map[string]time.Time{}
	}
	s.Setup[strings.ToLower(name)] = time.Now().UTC().Truncate(time.Second)
}

func (s *State) Touch(name string) {
	s.LastUsed[strings.ToLower(name)] = time.Now().UTC().Truncate(time.Second)
}

func (s *State) Forget(name string) {
	delete(s.LastUsed, strings.ToLower(name))
	delete(s.Setup, strings.ToLower(name))
}

func (s *State) Last(name string) time.Time {
	return s.LastUsed[strings.ToLower(name)]
}

func (s *State) Save() error {
	return s.SaveTo(StatePath())
}

func (s *State) SaveTo(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return WriteAtomic(path, append(data, '\n'), 0o600)
}

func Relative(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return ago(int(d.Minutes()), "min")
	case d < 24*time.Hour:
		return ago(int(d.Hours()), "hour")
	case d < 30*24*time.Hour:
		return ago(int(d.Hours()/24), "day")
	case d < 365*24*time.Hour:
		return ago(int(d.Hours()/24/30), "month")
	}
	return ago(int(d.Hours()/24/365), "year")
}

func ago(n int, unit string) string {
	if n == 1 {
		return "1 " + unit + " ago"
	}
	return strconv.Itoa(n) + " " + unit + "s ago"
}
