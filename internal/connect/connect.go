package connect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

const askpassScript = "#!/bin/sh\nprintf '%s' \"$SV_PASS\"\n"

func Args(c *config.Config, h *config.Host, remote []string) []string {
	r := c.Resolve(h)
	args := []string{"ssh", "-p", strconv.Itoa(r.Port)}
	switch r.Auth {
	case config.AuthKey:
		if r.Key != "" {
			args = append(args, "-i", config.ExpandHome(r.Key), "-o", "IdentitiesOnly=yes")
		}
	case config.AuthPassword:
		args = append(args,
			"-o", "PubkeyAuthentication=no",
			"-o", "NumberOfPasswordPrompts=1",
			"-o", "StrictHostKeyChecking=accept-new",
		)
	}
	if h.Jump != "" {
		args = append(args, "-J", h.Jump)
	}
	args = append(args, "--", r.User+"@"+h.Host)
	args = append(args, remote...)
	return args
}

func AskpassPath() string {
	return filepath.Join(filepath.Dir(config.Path()), "askpass.sh")
}

func PasswordEnv(password string) ([]string, error) {
	path := AskpassPath()
	if err := ensureAskpass(path); err != nil {
		return nil, err
	}
	return PasswordEnvFrom(os.Environ(), path, password), nil
}

func PasswordEnvFrom(base []string, askpass, password string) []string {
	env := make([]string, 0, len(base)+4)
	hasDisplay := false
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		switch key {
		case "SSH_ASKPASS", "SSH_ASKPASS_REQUIRE", "SV_PASS":
			continue
		case "DISPLAY":
			hasDisplay = true
		}
		env = append(env, kv)
	}
	env = append(env,
		"SSH_ASKPASS="+askpass,
		"SSH_ASKPASS_REQUIRE=force",
		"SV_PASS="+password,
	)
	if !hasDisplay {
		env = append(env, "DISPLAY=:0")
	}
	return env
}

func ensureAskpass(path string) error {
	if data, err := os.ReadFile(path); err == nil && string(data) == askpassScript {
		if st, err := os.Stat(path); err == nil && st.Mode().Perm() == 0o700 {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return config.WriteAtomic(path, []byte(askpassScript), 0o700)
}

var versionRe = regexp.MustCompile(`OpenSSH_(\d+)\.(\d+)`)

func CheckAskpassSupport() error {
	out, _ := exec.Command("ssh", "-V").CombinedOutput()
	return checkVersion(string(out))
}

func checkVersion(out string) error {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return nil
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major > 8 || (major == 8 && minor >= 4) {
		return nil
	}
	return fmt.Errorf("password auth needs OpenSSH 8.4 or newer, found %s.%s; upgrade ssh or switch the host to a key with ssh-copy-id", m[1], m[2])
}

func Exec(args, env []string) error {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	return syscall.Exec(path, args, env)
}
