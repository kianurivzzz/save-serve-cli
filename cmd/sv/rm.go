package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a host",
	Args:  cobra.ExactArgs(1),
	RunE:  runRm,
}

func init() {
	rmCmd.Flags().BoolP("yes", "y", false, "do not ask for confirmation")
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	h := cfg.Find(args[0])
	if h == nil {
		return fmt.Errorf("host %q not found", args[0])
	}
	var dependents []string
	for _, other := range cfg.Hosts {
		if strings.EqualFold(other.Jump, h.Name) {
			dependents = append(dependents, other.Name)
		}
	}
	if len(dependents) > 0 {
		return fmt.Errorf("host %q is used as jump by: %s", h.Name, strings.Join(dependents, ", "))
	}
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		r := cfg.Resolve(h)
		fmt.Fprintf(os.Stderr, "remove %s (%s@%s:%d)? [y/N] ", h.Name, r.User, h.Host, r.Port)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
		default:
			fmt.Fprintln(os.Stderr, "aborted")
			return nil
		}
	}
	name := h.Name
	cfg.Remove(name)
	if err := saveAndSync(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "removed %s\n", name)
	return nil
}
