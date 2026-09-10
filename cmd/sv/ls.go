package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
	"github.com/kianurivzzz/save-serve-cli/internal/tui"
)

var lsCmd = &cobra.Command{
	Use:               "ls",
	Short:             "List hosts",
	Args:              cobra.NoArgs,
	ValidArgsFunction: noCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		group, _ := cmd.Flags().GetString("group")
		tag, _ := cmd.Flags().GetString("tag")
		asJSON, _ := cmd.Flags().GetBool("json")
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		return listHosts(cfg, group, tag, asJSON)
	},
}

type hostJSON struct {
	Name     string     `json:"name"`
	Host     string     `json:"host"`
	User     string     `json:"user"`
	Port     int        `json:"port"`
	Auth     string     `json:"auth"`
	Key      string     `json:"key"`
	Group    string     `json:"group"`
	Tags     []string   `json:"tags"`
	Jump     string     `json:"jump"`
	Tmux     bool       `json:"tmux"`
	Note     string     `json:"note"`
	LastUsed *time.Time `json:"last_used"`
}

func init() {
	lsCmd.Flags().String("group", "", "only hosts in this group")
	lsCmd.Flags().String("tag", "", "only hosts with this tag")
	lsCmd.Flags().Bool("json", false, "print a JSON array instead of a table")
	lsCmd.RegisterFlagCompletionFunc("group", completeGroup)
	rootCmd.AddCommand(lsCmd)
}

func listHosts(cfg *config.Config, group, tag string, asJSON bool) error {
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
	state := config.LoadState()
	config.SortHosts(cfg, state, hosts)
	if asJSON {
		return printJSON(cfg, state, hosts)
	}
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "no hosts")
		return nil
	}
	now := time.Now()
	rows := make([][]string, 0, len(hosts))
	for _, h := range hosts {
		r := cfg.Resolve(h)
		rows = append(rows, []string{
			h.Name,
			fmt.Sprintf("%s@%s:%d", r.User, h.Host, r.Port),
			r.Auth,
			h.Group,
			strings.Join(h.Tags, ", "),
			config.Relative(state.Last(h.Name), now),
		})
	}
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(false).
		Headers("NAME", "TARGET", "AUTH", "GROUP", "TAGS", "LAST USED").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := lipgloss.NewStyle().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Bold(true).Foreground(lipgloss.Color("245"))
			}
			switch col {
			case 3:
				return s.Foreground(tui.GroupColor(cfg, hosts[row].Group))
			case 5:
				return s.Foreground(lipgloss.Color("245"))
			}
			return s
		})
	fmt.Println(t)
	return nil
}

func printJSON(cfg *config.Config, state *config.State, hosts []*config.Host) error {
	out := make([]hostJSON, 0, len(hosts))
	for _, h := range hosts {
		r := cfg.Resolve(h)
		j := hostJSON{
			Name:  h.Name,
			Host:  h.Host,
			User:  r.User,
			Port:  r.Port,
			Auth:  r.Auth,
			Key:   r.Key,
			Group: h.Group,
			Tags:  h.Tags,
			Jump:  h.Jump,
			Tmux:  h.Tmux,
			Note:  h.Note,
		}
		if j.Tags == nil {
			j.Tags = []string{}
		}
		if t := state.Last(h.Name); !t.IsZero() {
			j.LastUsed = &t
		}
		out = append(out, j)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func hasTag(h *config.Host, tag string) bool {
	for _, t := range h.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}
