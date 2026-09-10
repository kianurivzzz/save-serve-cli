package sshcfg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/connect"
)

const (
	beginMarker = "# >>> sv managed - do not edit, changes will be overwritten >>>"
	endMarker   = "# <<< sv managed <<<"
	beginPrefix = "# >>> sv managed"
	endPrefix   = "# <<< sv managed"
)

func Path() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

func Render(c *config.Config) string {
	var b strings.Builder
	b.WriteString(beginMarker + "\n")
	for i := range c.Hosts {
		h := &c.Hosts[i]
		r := c.Resolve(h)
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Host %s\n", h.Name)
		fmt.Fprintf(&b, "    HostName %s\n", h.Host)
		fmt.Fprintf(&b, "    User %s\n", r.User)
		fmt.Fprintf(&b, "    Port %d\n", r.Port)
		fmt.Fprintf(&b, "    ServerAliveInterval %d\n", connect.KeepAliveInterval)
		fmt.Fprintf(&b, "    ServerAliveCountMax %d\n", connect.KeepAliveCount)
		if r.Auth == config.AuthKey && r.Key != "" {
			fmt.Fprintf(&b, "    IdentityFile %s\n", r.Key)
			b.WriteString("    IdentitiesOnly yes\n")
		}
		if h.Jump != "" {
			fmt.Fprintf(&b, "    ProxyJump %s\n", h.Jump)
		}
	}
	b.WriteString(endMarker + "\n")
	return b.String()
}

func Merge(existing, block string) (string, error) {
	lines := strings.Split(existing, "\n")
	start, end := -1, -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if start == -1 {
			if strings.HasPrefix(t, beginPrefix) {
				start = i
			}
			continue
		}
		if strings.HasPrefix(t, endPrefix) {
			end = i
			break
		}
	}
	if start == -1 {
		if strings.TrimSpace(existing) == "" {
			return block, nil
		}
		if !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		return existing + "\n" + block, nil
	}
	if end == -1 {
		return "", errors.New("found begin marker of sv block but no end marker")
	}
	var out strings.Builder
	if start > 0 {
		out.WriteString(strings.Join(lines[:start], "\n"))
		out.WriteString("\n")
	}
	out.WriteString(block)
	if rest := lines[end+1:]; len(rest) > 0 {
		out.WriteString(strings.Join(rest, "\n"))
	}
	return out.String(), nil
}

func Write(c *config.Config) error {
	return WriteTo(Path(), c)
}

func WriteTo(path string, c *config.Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	merged, err := Merge(string(existing), Render(c))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if exists && merged == string(existing) {
		return nil
	}
	mode := os.FileMode(0o600)
	if exists {
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
		backup := path + ".sv-backup"
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(backup, existing, 0o600); err != nil {
				return fmt.Errorf("backup %s: %w", backup, err)
			}
		}
	}
	return config.WriteAtomic(path, []byte(merged), mode)
}
