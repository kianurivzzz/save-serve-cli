package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

var addFlags struct {
	user  string
	port  int
	key   string
	agent bool
	group string
	tags  []string
	jump  string
	note  string
}

var addCmd = &cobra.Command{
	Use:   "add <name> [user@]host[:port]",
	Short: "Add a host",
	Args:  cobra.ExactArgs(2),
	RunE:  runAdd,
}

func init() {
	f := addCmd.Flags()
	f.StringVar(&addFlags.user, "user", "", "ssh user")
	f.IntVar(&addFlags.port, "port", 0, "ssh port")
	f.StringVar(&addFlags.key, "key", "", "private key path (auth: key)")
	f.BoolVar(&addFlags.agent, "agent", false, "authenticate via ssh-agent (auth: agent)")
	f.StringVar(&addFlags.group, "group", "", "group name")
	f.StringArrayVar(&addFlags.tags, "tag", nil, "tag, repeatable")
	f.StringVar(&addFlags.jump, "jump", "", "name of the jump host (ProxyJump)")
	f.StringVar(&addFlags.note, "note", "", "free-form note")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if errors.Is(err, config.ErrNotFound) {
		cfg, err = config.New(), nil
	}
	if err != nil {
		return err
	}
	name := args[0]
	if cfg.Find(name) != nil {
		return fmt.Errorf("host %q already exists", name)
	}
	h, err := parseTarget(args[1])
	if err != nil {
		return err
	}
	h.Name = name
	if addFlags.user != "" {
		h.User = addFlags.user
	}
	if addFlags.port != 0 {
		h.Port = addFlags.port
	}
	h.Group = addFlags.group
	h.Tags = addFlags.tags
	h.Jump = addFlags.jump
	h.Note = addFlags.note

	switch {
	case addFlags.key != "" && addFlags.agent:
		return errors.New("--key and --agent are mutually exclusive")
	case addFlags.key != "":
		h.Auth = config.AuthKey
		h.Key = config.CollapseHome(addFlags.key)
		if !config.FileExists(config.ExpandHome(h.Key)) {
			fmt.Fprintf(os.Stderr, "warning: key %s does not exist\n", h.Key)
		}
	case addFlags.agent:
		h.Auth = config.AuthAgent
	case cfg.Defaults.Key != "" && config.FileExists(config.ExpandHome(cfg.Defaults.Key)):
		h.Auth = config.AuthKey
	default:
		h.Auth = config.AuthAgent
	}
	if h.Jump != "" && cfg.Find(h.Jump) == nil {
		return fmt.Errorf("jump host %q not found", h.Jump)
	}
	if h.Group != "" {
		cfg.EnsureGroup(h.Group)
	}
	cfg.Hosts = append(cfg.Hosts, h)
	if err := saveAndSync(cfg); err != nil {
		return err
	}
	r := cfg.Resolve(&h)
	fmt.Fprintf(os.Stderr, "added %s: %s@%s:%d, auth %s\n", h.Name, r.User, h.Host, r.Port, r.Auth)
	return nil
}

func parseTarget(s string) (config.Host, error) {
	var h config.Host
	if i := strings.LastIndex(s, "@"); i >= 0 {
		h.User = s[:i]
		s = s[i+1:]
	}
	if strings.Count(s, ":") == 1 {
		i := strings.Index(s, ":")
		port, err := strconv.Atoi(s[i+1:])
		if err != nil || port < 1 || port > 65535 {
			return h, fmt.Errorf("bad port in %q", s)
		}
		h.Port = port
		s = s[:i]
	}
	if s == "" {
		return h, errors.New("host is empty")
	}
	h.Host = s
	return h, nil
}
