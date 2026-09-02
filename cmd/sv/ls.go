package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List hosts",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		group, _ := cmd.Flags().GetString("group")
		tag, _ := cmd.Flags().GetString("tag")
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return listHosts(cfg, group, tag)
	},
}

func init() {
	lsCmd.Flags().String("group", "", "only hosts in this group")
	lsCmd.Flags().String("tag", "", "only hosts with this tag")
	rootCmd.AddCommand(lsCmd)
}

var palette = []string{"2", "3", "4", "5", "6", "1"}

var colorNames = map[string]string{
	"black": "0", "red": "1", "green": "2", "yellow": "3", "blue": "4",
	"magenta": "5", "cyan": "6", "white": "7", "gray": "245", "grey": "245",
	"orange": "208", "purple": "99", "pink": "205",
}

func listHosts(cfg *config.Config, group, tag string) error {
	hosts := make([]*config.Host, 0, len(cfg.Hosts))
	for i := range cfg.Hosts {
		h := &cfg.Hosts[i]
		if group != "" && !strings.EqualFold(h.Group, group) {
			continue
		}
		if tag != "" && !hasTag(h, tag) {
			continue
		}
		hosts = append(hosts, h)
	}
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "no hosts")
		return nil
	}
	sortHosts(cfg, hosts)
	rows := make([][]string, 0, len(hosts))
	for _, h := range hosts {
		r := cfg.Resolve(h)
		rows = append(rows, []string{
			h.Name,
			fmt.Sprintf("%s@%s:%d", r.User, h.Host, r.Port),
			r.Auth,
			h.Group,
			strings.Join(h.Tags, ", "),
		})
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(false).
		Headers("NAME", "TARGET", "AUTH", "GROUP", "TAGS").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("245"))
			}
			if col == 3 {
				return s.Foreground(groupColor(cfg, hosts[row].Group))
			}
			return s
		})
	fmt.Println(t)
	return nil
}

func hasTag(h *config.Host, tag string) bool {
	for _, t := range h.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func groupRank(cfg *config.Config, group string) int {
	if group == "" {
		return 1 << 30
	}
	if i := cfg.GroupIndex(group); i >= 0 {
		return i
	}
	return 1 << 29
}

func sortHosts(cfg *config.Config, hosts []*config.Host) {
	sort.SliceStable(hosts, func(i, j int) bool {
		a, b := hosts[i], hosts[j]
		ra, rb := groupRank(cfg, a.Group), groupRank(cfg, b.Group)
		if ra != rb {
			return ra < rb
		}
		if ga, gb := strings.ToLower(a.Group), strings.ToLower(b.Group); ga != gb {
			return ga < gb
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

func groupColor(cfg *config.Config, group string) lipgloss.TerminalColor {
	idx := cfg.GroupIndex(group)
	if idx < 0 {
		return lipgloss.NoColor{}
	}
	if c := cfg.Groups[idx].Color; c != "" {
		if code, ok := colorNames[strings.ToLower(c)]; ok {
			return lipgloss.Color(code)
		}
		return lipgloss.Color(c)
	}
	return lipgloss.Color(palette[idx%len(palette)])
}
