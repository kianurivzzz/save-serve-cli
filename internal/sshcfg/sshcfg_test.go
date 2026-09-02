package sshcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	key := filepath.Join(t.TempDir(), "id")
	os.WriteFile(key, nil, 0o600)
	return &config.Config{
		Defaults: config.Defaults{User: "root", Port: 22, Key: key},
		Hosts: []config.Host{
			{Name: "coolify", Host: "1.2.3.4", Auth: config.AuthKey},
			{Name: "CashCow", Host: "cashcow.example.com", Auth: config.AuthPassword},
			{Name: "bastion", Host: "b.example.com", User: "ops", Port: 2222, Auth: config.AuthAgent},
			{Name: "inner", Host: "10.0.0.5", Jump: "bastion", Auth: config.AuthAgent},
		},
	}
}

func TestRender(t *testing.T) {
	c := testConfig(t)
	got := Render(c)
	want := []string{
		beginMarker,
		"Host coolify\n    HostName 1.2.3.4\n    User root\n    Port 22\n    IdentityFile " + c.Defaults.Key + "\n    IdentitiesOnly yes\n",
		"Host CashCow\n    HostName cashcow.example.com\n    User root\n    Port 22\n",
		"Host bastion\n    HostName b.example.com\n    User ops\n    Port 2222\n",
		"Host inner\n    HostName 10.0.0.5\n    User root\n    Port 22\n    ProxyJump bastion\n",
		endMarker + "\n",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing:\n%s\nin:\n%s", w, got)
		}
	}
	if strings.Contains(got, "IdentityFile\n") || strings.Count(got, "IdentitiesOnly") != 1 {
		t.Errorf("identity lines leaked to non-key hosts:\n%s", got)
	}
	if !strings.HasPrefix(got, beginMarker+"\n") || !strings.HasSuffix(got, endMarker+"\n") {
		t.Errorf("markers wrong:\n%s", got)
	}
}

func TestMerge(t *testing.T) {
	block := beginMarker + "\nHost new\n" + endMarker + "\n"
	cases := []struct {
		name, existing, want string
	}{
		{"empty file", "", block},
		{"append with trailing newline", "Host a\n    User x\n", "Host a\n    User x\n\n" + block},
		{"append without trailing newline", "Host a", "Host a\n\n" + block},
		{"replace in middle", "Host a\n" + beginMarker + "\nHost old\n    User y\n" + endMarker + "\nHost z\n", "Host a\n" + block + "Host z\n"},
		{"replace at start", beginMarker + "\nHost old\n" + endMarker + "\nHost z\n", block + "Host z\n"},
		{"replace at end", "Host a\n\n" + beginMarker + "\nHost old\n" + endMarker + "\n", "Host a\n\n" + block},
		{"idempotent", block, block},
	}
	for _, tc := range cases {
		got, err := Merge(tc.existing, block)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s:\n--- got ---\n%q\n--- want ---\n%q", tc.name, got, tc.want)
		}
	}
	if _, err := Merge(beginMarker+"\nHost x\n", block); err == nil {
		t.Error("missing end marker should fail")
	}
}

func TestWriteTo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ssh", "config")
	c := testConfig(t)

	if err := WriteTo(path, c); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o", st.Mode().Perm())
	}
	if _, err := os.Stat(path + ".sv-backup"); err == nil {
		t.Error("backup must not be created when config did not exist")
	}

	os.WriteFile(path, []byte("Host mine\n    User me\n"), 0o644)
	os.Chmod(path, 0o644)
	if err := WriteTo(path, c); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".sv-backup")
	if err != nil || string(backup) != "Host mine\n    User me\n" {
		t.Errorf("backup = %q, err %v", backup, err)
	}
	st, _ = os.Stat(path)
	if st.Mode().Perm() != 0o644 {
		t.Errorf("existing mode should be preserved, got %o", st.Mode().Perm())
	}
	first, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(first), "Host mine\n    User me\n\n"+beginMarker) {
		t.Errorf("user content lost:\n%s", first)
	}

	c.Hosts = c.Hosts[:1]
	if err := WriteTo(path, c); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if strings.Count(string(second), beginMarker) != 1 || strings.Contains(string(second), "Host CashCow") {
		t.Errorf("block not replaced:\n%s", second)
	}
	backup2, _ := os.ReadFile(path + ".sv-backup")
	if string(backup2) != string(backup) {
		t.Error("backup must only be written once")
	}
}
