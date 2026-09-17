package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, home, shell string) string {
	t.Helper()
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader(Script)
	cmd.Env = []string{"HOME=" + home, "SHELL=" + shell, "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	return string(out)
}

func TestScriptBash(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".bashrc")
	os.WriteFile(rc, []byte("export PATH=$PATH:/opt/bin"), 0o644)
	out := run(t, home, "/bin/bash")
	if !strings.Contains(out, "history: bash, block written to ~/.bashrc") {
		t.Errorf("output: %s", out)
	}
	data, _ := os.ReadFile(rc)
	got := string(data)
	want := "export PATH=$PATH:/opt/bin\n\n" + beginMarker + "\n" + bashBlock + "\n" + endMarker + "\n"
	if got != want {
		t.Errorf("--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	run(t, home, "/bin/bash")
	data, _ = os.ReadFile(rc)
	if string(data) != want {
		t.Errorf("second run must be idempotent:\n%s", data)
	}
	if err := exec.Command("bash", "-n", rc).Run(); err != nil {
		t.Errorf("bash -n: %v", err)
	}
}

func TestBashHistorySurvivesKill(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".bashrc")
	os.WriteFile(rc, []byte("HISTFILE=/dev/null\nexport PROMPT_COMMAND='echo; echo; echo'\n"), 0o644)
	os.WriteFile(filepath.Join(home, ".bash_history"), []byte("echo old\n"), 0o600)
	run(t, home, "/bin/bash")
	cmd := exec.Command("bash", "--rcfile", rc, "-i")
	cmd.Stdin = strings.NewReader("echo sv-marker\nkill -9 $$\n")
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	out, _ := cmd.CombinedOutput()
	data, err := os.ReadFile(filepath.Join(home, ".bash_history"))
	if err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if !strings.Contains(string(data), "echo sv-marker") {
		t.Errorf("command must reach the history file before the shell dies:\n%s", data)
	}
}

func TestScriptZshNoRc(t *testing.T) {
	home := t.TempDir()
	run(t, home, "/usr/bin/zsh")
	data, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	want := beginMarker + "\n" + zshBlock + "\n" + endMarker + "\n"
	if string(data) != want {
		t.Errorf("--- got ---\n%s\n--- want ---\n%s", data, want)
	}
}

func TestScriptReplacesOldBlock(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".bashrc")
	old := "alias ll='ls -l'\n\n" + beginMarker + "\nOLD\n" + endMarker + "\nalias la='ls -a'\n"
	os.WriteFile(rc, []byte(old), 0o644)
	run(t, home, "/bin/bash")
	data, _ := os.ReadFile(rc)
	got := string(data)
	if strings.Contains(got, "OLD") || strings.Count(got, beginMarker) != 1 {
		t.Errorf("old block must be replaced:\n%s", got)
	}
	if !strings.HasPrefix(got, "alias ll='ls -l'\n\nalias la='ls -a'\n\n"+beginMarker) {
		t.Errorf("surrounding lines must be kept:\n%s", got)
	}
}

func TestScriptUnknownShell(t *testing.T) {
	cmd := exec.Command("sh", "-s")
	cmd.Stdin = strings.NewReader(Script)
	cmd.Env = []string{"HOME=" + t.TempDir(), "SHELL=/usr/bin/fish", "PATH=" + os.Getenv("PATH")}
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "fish") {
		t.Errorf("want failure for fish, got %v: %s", err, out)
	}
}
