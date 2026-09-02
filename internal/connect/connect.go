package connect

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

func Args(c *config.Config, h *config.Host, remote []string) ([]string, error) {
	r := c.Resolve(h)
	if r.Auth == config.AuthPassword {
		return nil, fmt.Errorf("host %q uses password auth, which is not supported yet; run `ssh-copy-id %s@%s` and switch it to key", h.Name, r.User, h.Host)
	}
	args := []string{"ssh", "-p", strconv.Itoa(r.Port)}
	if r.Auth == config.AuthKey && r.Key != "" {
		args = append(args, "-i", config.ExpandHome(r.Key), "-o", "IdentitiesOnly=yes")
	}
	if h.Jump != "" {
		args = append(args, "-J", h.Jump)
	}
	args = append(args, "--", r.User+"@"+h.Host)
	args = append(args, remote...)
	return args, nil
}

func Exec(args []string) error {
	path, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	return syscall.Exec(path, args, os.Environ())
}
