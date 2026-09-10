package connect

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

func TestArgs(t *testing.T) {
	key := filepath.Join(t.TempDir(), "id")
	os.WriteFile(key, nil, 0o600)
	c := &config.Config{
		Defaults: config.Defaults{User: "root", Port: 22, Key: key},
		Hosts: []config.Host{
			{Name: "k", Host: "1.2.3.4", Auth: config.AuthKey},
			{Name: "a", Host: "2.2.2.2", User: "ops", Port: 2222, Auth: config.AuthAgent},
			{Name: "j", Host: "3.3.3.3", Auth: config.AuthAgent, Jump: "a"},
			{Name: "p", Host: "4.4.4.4", Auth: config.AuthPassword},
			{Name: "t", Host: "5.5.5.5", Auth: config.AuthAgent, Tmux: true},
		},
	}
	cases := []struct {
		host   string
		remote []string
		want   string
	}{
		{"k", nil, "ssh -p 22 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -i " + key + " -o IdentitiesOnly=yes -- root@1.2.3.4"},
		{"a", []string{"docker", "ps"}, "ssh -p 2222 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -- ops@2.2.2.2 docker ps"},
		{"j", []string{"-weird"}, "ssh -p 22 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -J a -- root@3.3.3.3 -weird"},
		{"p", nil, "ssh -p 22 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -o PubkeyAuthentication=no -o NumberOfPasswordPrompts=1 -o StrictHostKeyChecking=accept-new -- root@4.4.4.4"},
		{"t", nil, "ssh -p 22 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -t -- root@5.5.5.5 " + tmuxCommand},
		{"t", []string{"uptime"}, "ssh -p 22 -o ServerAliveInterval=15 -o ServerAliveCountMax=4 -- root@5.5.5.5 uptime"},
	}
	for _, tc := range cases {
		got := Args(c, c.Find(tc.host), tc.remote)
		if s := strings.Join(got, " "); s != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.host, s, tc.want)
		}
	}
}

func TestPasswordEnvFrom(t *testing.T) {
	base := []string{"HOME=/h", "SSH_ASKPASS=/old", "SV_PASS=old", "SSH_ASKPASS_REQUIRE=prefer"}
	env := PasswordEnvFrom(base, "/cfg/askpass.sh", "p w")
	want := []string{"HOME=/h", "SSH_ASKPASS=/cfg/askpass.sh", "SSH_ASKPASS_REQUIRE=force", "SV_PASS=p w", "DISPLAY=:0"}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q", env)
	}
	env = PasswordEnvFrom([]string{"DISPLAY=:1"}, "/a", "x")
	if strings.Count(strings.Join(env, "\n"), "DISPLAY=") != 1 || env[0] != "DISPLAY=:1" {
		t.Errorf("existing DISPLAY must be kept: %q", env)
	}
}

func TestEnsureAskpass(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sv", "askpass.sh")
	if err := ensureAskpass(path); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o700 {
		t.Errorf("mode = %o", st.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if string(data) != askpassScript || strings.Contains(string(data), "SV_PASS=") {
		t.Errorf("content:\n%s", data)
	}
	os.WriteFile(path, []byte("tampered"), 0o755)
	if err := ensureAskpass(path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	st, _ = os.Stat(path)
	if string(data) != askpassScript || st.Mode().Perm() != 0o700 {
		t.Errorf("tampered script must be rewritten: %q %o", data, st.Mode().Perm())
	}
}

func TestCheckVersion(t *testing.T) {
	cases := map[string]bool{
		"OpenSSH_10.3p1, LibreSSL 3.3.6":                    true,
		"OpenSSH_8.4p1 Debian-5, OpenSSL 1.1.1k":            true,
		"OpenSSH_8.3p1 Ubuntu-1ubuntu0.1, OpenSSL 1.1.1f":   false,
		"OpenSSH_7.9p1 Raspbian-10+deb10u2, OpenSSL 1.1.1d": false,
		"OpenSSH_9.6p1 Ubuntu-3ubuntu13.5, OpenSSL 3.0.13":  true,
		"something else": true,
	}
	for out, ok := range cases {
		err := checkVersion(out)
		if ok && err != nil {
			t.Errorf("%q: unexpected %v", out, err)
		}
		if !ok && (err == nil || !strings.Contains(err.Error(), "8.4")) {
			t.Errorf("%q: want version error, got %v", out, err)
		}
	}
}

func TestTmuxCommand(t *testing.T) {
	bin := t.TempDir()
	fakeShell := filepath.Join(bin, "fakesh")
	os.WriteFile(fakeShell, []byte("#!/bin/sh\necho SHELL \"$@\"\n"), 0o755)
	for _, sh := range []string{"sh", "bash", "zsh"} {
		if _, err := exec.LookPath(sh); err != nil {
			continue
		}
		run := func(path string) string {
			c := exec.Command(sh, "-c", tmuxCommand)
			c.Env = []string{"PATH=" + path, "SHELL=" + fakeShell}
			out, err := c.CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v: %s", sh, err, out)
			}
			return string(out)
		}
		out := run(t.TempDir())
		if !strings.Contains(out, "tmux is not installed") || !strings.Contains(out, "SHELL -l") {
			t.Errorf("%s without tmux: %q", sh, out)
		}
		os.WriteFile(filepath.Join(bin, "tmux"), []byte("#!/bin/sh\necho TMUX \"$@\"\n"), 0o755)
		out = run(bin)
		if out != "TMUX new-session -A -s main\n" {
			t.Errorf("%s with tmux: %q", sh, out)
		}
		os.Remove(filepath.Join(bin, "tmux"))
	}
}
