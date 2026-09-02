package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

var palette = []string{"2", "3", "4", "5", "6", "1"}

var colorNames = map[string]string{
	"black": "0", "red": "1", "green": "2", "yellow": "3", "blue": "4",
	"magenta": "5", "cyan": "6", "white": "7", "gray": "245", "grey": "245",
	"orange": "208", "purple": "99", "pink": "205",
}

var (
	dim      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	selected = lipgloss.NewStyle().Bold(true)
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func colorCode(name string) string {
	if code, ok := colorNames[strings.ToLower(name)]; ok {
		return code
	}
	return name
}

func KnownColor(name string) bool {
	_, ok := colorNames[strings.ToLower(name)]
	return ok
}

func GroupColor(c *config.Config, group string) lipgloss.TerminalColor {
	idx := c.GroupIndex(group)
	if idx < 0 {
		return lipgloss.NoColor{}
	}
	if col := c.Groups[idx].Color; col != "" {
		return lipgloss.Color(colorCode(col))
	}
	return lipgloss.Color(palette[idx%len(palette)])
}

func AuthIcon(auth string) string {
	switch auth {
	case config.AuthKey:
		return "🔑"
	case config.AuthPassword:
		return "🔐"
	default:
		return "🔒"
	}
}
