package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/tui"
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "Manage groups",
}

var groupLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List groups",
	Args:  cobra.NoArgs,
	RunE:  runGroupLs,
}

var groupAddCmd = &cobra.Command{
	Use:               "add <name>",
	Short:             "Add a group",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: noCompletion,
	RunE:              runGroupAdd,
}

var groupRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a group, its hosts stay without a group",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return noCompletion(cmd, args, toComplete)
		}
		return completeGroup(cmd, args, toComplete)
	},
	RunE: runGroupRm,
}

var groupMvCmd = &cobra.Command{
	Use:   "mv <host> <group>",
	Short: "Move a host into a group",
	Args:  cobra.ExactArgs(2),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeHost(cmd, args, toComplete)
		case 1:
			return completeGroup(cmd, args, toComplete)
		}
		return noCompletion(cmd, args, toComplete)
	},
	RunE: runGroupMv,
}

func init() {
	groupAddCmd.Flags().String("color", "", "color name (green, orange, ...), ANSI code or #hex")
	groupCmd.AddCommand(groupLsCmd, groupAddCmd, groupRmCmd, groupMvCmd)
	rootCmd.AddCommand(groupCmd)
}

func runGroupLs(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Groups) == 0 {
		fmt.Fprintln(os.Stderr, "no groups")
		return nil
	}
	counts := map[string]int{}
	for _, h := range cfg.Hosts {
		counts[strings.ToLower(h.Group)]++
	}
	rows := make([][]string, 0, len(cfg.Groups))
	for _, g := range cfg.Groups {
		rows = append(rows, []string{g.Name, g.Color, fmt.Sprint(counts[strings.ToLower(g.Name)])})
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(false).
		Headers("NAME", "COLOR", "HOSTS").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("245"))
			}
			if col == 0 {
				return s.Foreground(tui.GroupColor(cfg, cfg.Groups[row].Name))
			}
			return s
		})
	fmt.Println(t)
	if n := counts[""]; n > 0 {
		fmt.Fprintf(os.Stderr, "without a group: %d\n", n)
	}
	return nil
}

func runGroupAdd(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	name := args[0]
	if cfg.GroupIndex(name) >= 0 {
		return fmt.Errorf("group %q already exists", name)
	}
	color, _ := cmd.Flags().GetString("color")
	if err := checkColor(color); err != nil {
		return err
	}
	cfg.Groups = append(cfg.Groups, config.Group{Name: name, Color: color})
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "added group %s\n", name)
	return nil
}

func runGroupRm(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	name := args[0]
	moved := cfg.RemoveGroup(name)
	if moved < 0 {
		return fmt.Errorf("group %q not found", name)
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "removed group %s, hosts left without a group: %d\n", name, moved)
	return nil
}

func runGroupMv(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	h := cfg.Find(args[0])
	if h == nil {
		return fmt.Errorf("host %q not found", args[0])
	}
	group := args[1]
	if idx := cfg.GroupIndex(group); idx >= 0 {
		group = cfg.Groups[idx].Name
	} else {
		cfg.EnsureGroup(group)
	}
	h.Group = group
	if err := saveAndSync(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "moved %s to group %s\n", h.Name, group)
	return nil
}

func checkColor(c string) error {
	if c == "" || tui.KnownColor(c) {
		return nil
	}
	if strings.HasPrefix(c, "#") && (len(c) == 7 || len(c) == 4) {
		return nil
	}
	n := 0
	if _, err := fmt.Sscanf(c, "%d", &n); err == nil && n >= 0 && n <= 255 {
		return nil
	}
	return fmt.Errorf("unknown color %q: use a name like green or orange, an ANSI code 0-255 or #rrggbb", c)
}
