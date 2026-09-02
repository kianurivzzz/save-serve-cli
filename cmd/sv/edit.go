package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

var editCmd = &cobra.Command{
	Use:               "edit [name]",
	Short:             "Open hosts.yaml in $EDITOR, then validate and sync",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeHostFirstArg,
	RunE:              runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	path := config.Path()
	if _, err := os.Stat(path); err != nil {
		return err
	}
	line := 0
	if len(args) == 1 {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		h := cfg.Find(args[0])
		if h == nil {
			return fmt.Errorf("host %q not found", args[0])
		}
		line = hostLine(path, h.Name)
	}
	c := editorCommand(path, line)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", c.Path, err)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return syncSSHConfig(cfg)
}

func editorCommand(path string, line int) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	args := parts[1:]
	if line > 0 && supportsLineArg(parts[0]) {
		args = append(args, fmt.Sprintf("+%d", line))
	}
	args = append(args, path)
	return exec.Command(parts[0], args...)
}

func supportsLineArg(editor string) bool {
	switch filepath.Base(editor) {
	case "vi", "vim", "nvim", "nano", "emacs", "micro", "hx":
		return true
	}
	return false
}

func hostLine(path, name string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		t := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(t, "- name:") {
			continue
		}
		v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "- name:")), `"'`)
		if strings.EqualFold(v, name) {
			return n
		}
	}
	return 0
}
