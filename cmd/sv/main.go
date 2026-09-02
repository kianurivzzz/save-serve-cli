package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/connect"
	"github.com/kianurivzzz/save-serve-cli/internal/keychain"
	"github.com/kianurivzzz/save-serve-cli/internal/sshcfg"
	"github.com/kianurivzzz/save-serve-cli/internal/tui"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "sv [host] [command...]",
	Short: "SSH host manager",
	Long: `sv keeps your SSH hosts in ~/.config/sv/hosts.yaml and mirrors them
into a managed block of ~/.ssh/config, so both "sv <name>" and "ssh <name>" work.

  sv                      pick a host interactively
  sv <name>               connect
  sv <name> <command...>  run a command and exit
  sv <name> -- <command>  same, for commands that start with a dash`,
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: completeHostFirstArg,
	Version:           version,
	SilenceUsage:      true,
	SilenceErrors:     true,
	RunE:              runRoot,
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
		if !isTerminal() {
			return listHosts(cfg, "", "", false)
		}
		return pickAndConnect(cfg, "", nil)
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
			if isTerminal() {
				return pickAndConnect(cfg, name, remote)
			}
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return fmt.Errorf("%q matches several hosts: %s", name, strings.Join(names, ", "))
		}
	}
	return connectTo(cfg, h, remote)
}

func pickAndConnect(cfg *config.Config, filter string, remote []string) error {
	h, err := tui.Run(tui.Options{
		Config: cfg,
		State:  config.LoadState(),
		Filter: filter,
		Editor: func() *exec.Cmd { return editorCommand(config.Path(), 0) },
		Reload: reloadAndSync,
	})
	if err != nil {
		return err
	}
	if h == nil {
		return nil
	}
	return connectTo(cfg, h, remote)
}

func connectTo(cfg *config.Config, h *config.Host, remote []string) error {
	argv := connect.Args(cfg, h, remote)
	env := os.Environ()
	if cfg.Resolve(h).Auth == config.AuthPassword {
		if err := connect.CheckAskpassSupport(); err != nil {
			return err
		}
		store, err := keychain.Open(cfg.Defaults.SecretStore)
		if err != nil {
			return err
		}
		pw, err := store.Get(h.Name)
		if errors.Is(err, keychain.ErrNotFound) {
			return fmt.Errorf("no password stored for %q, run: sv passwd %s", h.Name, h.Name)
		}
		if err != nil {
			return err
		}
		env, err = connect.PasswordEnv(pw)
		if err != nil {
			return err
		}
	}
	st := config.LoadState()
	st.Touch(h.Name)
	if err := st.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot save %s: %v\n", config.StatePath(), err)
	}
	return connect.Exec(argv, env)
}

func isTerminal() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		if !term.IsTerminal(int(f.Fd())) {
			return false
		}
	}
	return true
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("%s does not exist yet\n  import from ~/.ssh/config:  sv import ssh-config\n  import a Termius export:    sv import termius hosts.csv\n  or add a host by hand:      sv add <name> [user@]host[:port]", config.CollapseHome(config.Path()))
	}
	if err != nil {
		return nil, fmt.Errorf("%w\n  fix it with: sv edit", err)
	}
	return cfg, err
}

func loadOrNewConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotFound) {
		return config.New(), nil
	}
	return cfg, err
}

func reloadAndSync() (*config.Config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if err := sshcfg.Write(cfg); err != nil {
		return nil, fmt.Errorf("sync %s: %w", sshcfg.Path(), err)
	}
	return cfg, nil
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
	fmt.Fprintf(os.Stderr, "synced %d hosts to %s\n", len(cfg.Hosts), config.CollapseHome(sshcfg.Path()))
	return nil
}

func openStore(cfg *config.Config, flag string) (keychain.Store, error) {
	kind := flag
	if kind == "" {
		kind = cfg.Defaults.SecretStore
	}
	return keychain.Open(kind)
}

func completeHostFirstArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return completeHost(cmd, args, toComplete)
}

func completeHost(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, h := range cfg.Hosts {
		if strings.HasPrefix(strings.ToLower(h.Name), strings.ToLower(toComplete)) {
			out = append(out, h.Name+"\t"+h.Host)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func completeGroup(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	var out []string
	for _, g := range cfg.Groups {
		if strings.HasPrefix(strings.ToLower(g.Name), strings.ToLower(toComplete)) {
			out = append(out, g.Name)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func noCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}
