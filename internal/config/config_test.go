package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "hosts.yaml")
	c := &Config{
		Defaults: Defaults{User: "root", Port: 22, Key: "~/.ssh/id_ed25519"},
		Groups:   []Group{{Name: "мой", Color: "green"}},
		Hosts: []Host{
			{Name: "coolify", Host: "1.2.3.4", Auth: AuthKey, Group: "мой", Tags: []string{"prod", "docker"}, Note: "all side projects"},
			{Name: "bastion", Host: "b.example.com", User: "ops", Port: 2222, Auth: AuthAgent},
			{Name: "inner", Host: "10.0.0.5", Jump: "bastion"},
		},
	}
	if err := c.SaveTo(path); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 600", st.Mode().Perm())
	}
	dst, _ := os.Stat(filepath.Dir(path))
	if dst.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 700", dst.Mode().Perm())
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "tags: [prod, docker]") {
		t.Errorf("tags should be flow style, got:\n%s", raw)
	}
	if !strings.Contains(string(raw), "version: 1") {
		t.Errorf("version missing:\n%s", raw)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hosts) != 3 || got.Hosts[0].Name != "coolify" || got.Hosts[1].Port != 2222 || got.Hosts[2].Jump != "bastion" {
		t.Errorf("round trip mismatch: %+v", got.Hosts)
	}
	if got.Groups[0].Color != "green" {
		t.Errorf("group color lost: %+v", got.Groups)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := LoadFrom(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLoadBadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.yaml")
	os.WriteFile(path, []byte("hosts:\n  - name: x\n   host: [oops\n"), 0o600)
	_, err := LoadFrom(path)
	if err == nil || !strings.Contains(err.Error(), "line") {
		t.Errorf("want yaml error with line number, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"dup case-insensitive", Config{Hosts: []Host{{Name: "A", Host: "h"}, {Name: "a", Host: "h"}}}, "duplicate"},
		{"empty host", Config{Hosts: []Host{{Name: "a"}}}, "host is empty"},
		{"bad auth", Config{Hosts: []Host{{Name: "a", Host: "h", Auth: "magic"}}}, "auth must be"},
		{"unknown jump", Config{Hosts: []Host{{Name: "a", Host: "h", Jump: "nope"}}}, "not a known host"},
		{"self jump", Config{Hosts: []Host{{Name: "a", Host: "h", Jump: "A"}}}, "itself"},
		{"space in name", Config{Hosts: []Host{{Name: "a b", Host: "h"}}}, "spaces"},
		{"bad port", Config{Hosts: []Host{{Name: "a", Host: "h", Port: 70000}}}, "out of range"},
		{"ok", Config{Hosts: []Host{{Name: "a", Host: "h"}, {Name: "b", Host: "h", Jump: "A"}}}, ""},
	}
	for _, tc := range cases {
		err := tc.cfg.Validate()
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want containing %q", tc.name, err, tc.want)
		}
	}
}

func TestResolve(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id")
	os.WriteFile(key, nil, 0o600)
	c := &Config{Defaults: Defaults{User: "root", Port: 22, Key: key}}

	r := c.Resolve(&Host{Name: "a", Host: "h"})
	if r.User != "root" || r.Port != 22 || r.Auth != AuthKey || r.Key != key {
		t.Errorf("defaults: %+v", r)
	}
	r = c.Resolve(&Host{Name: "a", Host: "h", User: "ops", Port: 2222, Auth: AuthAgent})
	if r.User != "ops" || r.Port != 2222 || r.Auth != AuthAgent || r.Key != "" {
		t.Errorf("overrides: %+v", r)
	}
	r = c.Resolve(&Host{Name: "a", Host: "h", Key: "~/.ssh/other"})
	if r.Auth != AuthKey || r.Key != "~/.ssh/other" {
		t.Errorf("explicit key: %+v", r)
	}
	noKey := &Config{Defaults: Defaults{Key: "/does/not/exist"}}
	r = noKey.Resolve(&Host{Name: "a", Host: "h"})
	if r.Auth != AuthAgent || r.Key != "" {
		t.Errorf("missing default key should fall back to agent: %+v", r)
	}
	if r.Port != 22 {
		t.Errorf("port fallback = %d", r.Port)
	}
}

func TestFindAndMatch(t *testing.T) {
	c := &Config{Hosts: []Host{
		{Name: "coolify", Host: "h"},
		{Name: "CashCow", Host: "h"},
		{Name: "crown-prod", Host: "h"},
	}}
	if h := c.Find("cashcow"); h == nil || h.Name != "CashCow" {
		t.Errorf("Find case-insensitive failed: %v", h)
	}
	if h := c.Find("cash"); h != nil {
		t.Errorf("Find must be exact, got %v", h)
	}
	names := func(hs []*Host) []string {
		var out []string
		for _, h := range hs {
			out = append(out, h.Name)
		}
		return out
	}
	if got := names(c.Match("cool")); len(got) != 1 || got[0] != "coolify" {
		t.Errorf("substring: %v", got)
	}
	if got := names(c.Match("c")); len(got) != 3 {
		t.Errorf("substring all: %v", got)
	}
	if got := names(c.Match("cwp")); len(got) != 1 || got[0] != "crown-prod" {
		t.Errorf("subsequence: %v", got)
	}
	if got := c.Match("zzz"); len(got) != 0 {
		t.Errorf("no match: %v", got)
	}
}

func TestCollapseHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := CollapseHome(filepath.Join(home, ".ssh", "id")); got != "~/.ssh/id" {
		t.Errorf("got %q", got)
	}
	if got := CollapseHome("/etc/ssh/key"); got != "/etc/ssh/key" {
		t.Errorf("got %q", got)
	}
	if got := ExpandHome("~/.ssh/id"); got != filepath.Join(home, ".ssh", "id") {
		t.Errorf("expand got %q", got)
	}
}

func TestRemoveAndEnsureGroup(t *testing.T) {
	c := &Config{Hosts: []Host{{Name: "a", Host: "h"}, {Name: "b", Host: "h"}}}
	if !c.Remove("A") || len(c.Hosts) != 1 || c.Hosts[0].Name != "b" {
		t.Errorf("remove: %+v", c.Hosts)
	}
	if c.Remove("zzz") {
		t.Error("remove of missing host returned true")
	}
	c.EnsureGroup("prod")
	c.EnsureGroup("PROD")
	if len(c.Groups) != 1 {
		t.Errorf("groups: %+v", c.Groups)
	}
}

func TestValidateGroupsAndStore(t *testing.T) {
	c := Config{Groups: []Group{{Name: "a"}, {Name: "A"}}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("dup group: %v", err)
	}
	c = Config{Defaults: Defaults{SecretStore: "vault"}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "secret_store") {
		t.Errorf("bad store: %v", err)
	}
	c = Config{Defaults: Defaults{SecretStore: "file"}}
	if err := c.Validate(); err != nil {
		t.Errorf("file store: %v", err)
	}
}

func TestRemoveGroup(t *testing.T) {
	c := &Config{
		Groups: []Group{{Name: "a"}, {Name: "b"}},
		Hosts:  []Host{{Name: "x", Host: "h", Group: "A"}, {Name: "y", Host: "h", Group: "b"}},
	}
	if n := c.RemoveGroup("a"); n != 1 || len(c.Groups) != 1 || c.Hosts[0].Group != "" || c.Hosts[1].Group != "b" {
		t.Errorf("remove: n=%d groups=%+v hosts=%+v", n, c.Groups, c.Hosts)
	}
	if n := c.RemoveGroup("zzz"); n != -1 {
		t.Errorf("missing group: %d", n)
	}
}

func TestSortHosts(t *testing.T) {
	c := &Config{
		Groups: []Group{{Name: "work"}, {Name: "home"}},
		Hosts: []Host{
			{Name: "z-none", Host: "h"},
			{Name: "b-home", Host: "h", Group: "home"},
			{Name: "old-work", Host: "h", Group: "work"},
			{Name: "new-work", Host: "h", Group: "work"},
			{Name: "a-unknown", Host: "h", Group: "other"},
		},
	}
	s := &State{LastUsed: map[string]time.Time{
		"old-work": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		"new-work": time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}}
	hosts := make([]*Host, len(c.Hosts))
	for i := range c.Hosts {
		hosts[i] = &c.Hosts[i]
	}
	SortHosts(c, s, hosts)
	var got []string
	for _, h := range hosts {
		got = append(got, h.Name)
	}
	want := "new-work old-work b-home a-unknown z-none"
	if strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}

func TestDefaultAuth(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id_ed25519")
	c := &Config{Defaults: Defaults{Key: key}}
	if got := c.DefaultAuth(); got != AuthAgent {
		t.Errorf("missing key file: %q, want agent", got)
	}
	os.WriteFile(key, []byte("k"), 0o600)
	if got := c.DefaultAuth(); got != AuthKey {
		t.Errorf("existing key file: %q, want key", got)
	}
	if got := (&Config{}).DefaultAuth(); got != AuthAgent {
		t.Errorf("no default key: %q, want agent", got)
	}
}
