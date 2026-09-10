package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/connect"
	"github.com/kianurivzzz/save-serve-cli/internal/setup"
)

const sshConnectionFailed = 255

var setupCmd = &cobra.Command{
	Use:   "setup <name>",
	Short: "Configure shell history on the server so commands are never lost",
	Long: `sv setup writes a managed block into ~/.bashrc or ~/.zshrc on the server:
history is appended after every command instead of on shell exit, so a dropped
connection or several parallel sessions no longer overwrite it. It also reports
whether tmux is installed, which "tmux: true" in hosts.yaml needs.

Run it again at any time, the block is replaced in place.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeHostFirstArg,
	RunE:              runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	h := cfg.Find(args[0])
	if h == nil {
		return fmt.Errorf("host %q not found", args[0])
	}
	if err := remoteSetup(cfg, h); err != nil {
		return err
	}
	st := config.LoadState()
	st.MarkSetup(h.Name)
	if err := st.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot save %s: %v\n", config.StatePath(), err)
	}
	fmt.Fprintln(os.Stderr, "done, takes effect in new sessions")
	return nil
}

func remoteSetup(cfg *config.Config, h *config.Host) error {
	env, err := connectEnv(cfg, h)
	if err != nil {
		return err
	}
	argv := connect.Args(cfg, h, []string{"sh", "-s"})
	c := exec.Command(argv[0], argv[1:]...)
	c.Env = env
	c.Stdin = strings.NewReader(setup.Script)
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("setup %s: %w", h.Name, err)
	}
	return nil
}

func autoSetup(cfg *config.Config, h *config.Host, st *config.State) {
	if !cfg.Defaults.AutoSetupEnabled() || st.SetupDone(h.Name) {
		return
	}
	fmt.Fprintf(os.Stderr, "first connection to %s, setting up shell history (disable with defaults.auto_setup: false)\n", h.Name)
	err := remoteSetup(cfg, h)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == sshConnectionFailed {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v, run sv setup %s to retry\n", err, h.Name)
	}
	st.MarkSetup(h.Name)
}
