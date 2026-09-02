package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/connect"
	"github.com/kianurivzzz/save-serve-cli/internal/sshcfg"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "sv [host] [command...]",
	Short: "SSH host manager",
	Long: `sv keeps your SSH hosts in ~/.config/sv/hosts.yaml and mirrors them
into a managed block of ~/.ssh/config, so both "sv <name>" and "ssh <name>" work.

  sv                      list hosts
  sv <name>               connect
  sv <name> <command...>  run a command and exit
  sv <name> -- <command>  same, for commands that start with a dash`,
	Args:          cobra.ArbitraryArgs,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runRoot,
}

func init() {
	rootCmd.Flags().SetInterspersed(false)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sv:", err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return listHosts(cfg, "", "")
	}
	name, remote := args[0], args[1:]
	if len(remote) > 0 && remote[0] == "--" {
		remote = remote[1:]
	}
	h := cfg.Find(name)
	if h == nil {
		matches := cfg.Match(name)
		switch len(matches) {
		case 0:
			return fmt.Errorf("host %q not found", name)
		case 1:
			h = matches[0]
		default:
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return fmt.Errorf("%q matches several hosts: %s", name, strings.Join(names, ", "))
		}
	}
	argv, err := connect.Args(cfg, h, remote)
	if err != nil {
		return err
	}
	return connect.Exec(argv)
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("%s does not exist yet\n  import from ~/.ssh/config:  sv import ssh-config\n  or add a host by hand:      sv add <name> [user@]host[:port]", config.Path())
	}
	return cfg, err
}

func saveAndSync(cfg *config.Config) error {
	if err := cfg.Save(); err != nil {
		return err
	}
	return syncSSHConfig(cfg)
}

func syncSSHConfig(cfg *config.Config) error {
	if err := sshcfg.Write(cfg); err != nil {
		return fmt.Errorf("sync %s: %w", sshcfg.Path(), err)
	}
	fmt.Fprintf(os.Stderr, "synced %d hosts to %s\n", len(cfg.Hosts), sshcfg.Path())
	return nil
}
