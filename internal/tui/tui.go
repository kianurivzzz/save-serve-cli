package tui

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

type Options struct {
	Config *config.Config
	State  *config.State
	Filter string
	Editor func() *exec.Cmd
	Reload func() (*config.Config, error)
}

func Run(o Options) (*config.Host, error) {
	m := newModel(o)
	p := tea.NewProgram(m, tea.WithAltScreen())
	out, err := p.Run()
	if err != nil {
		return nil, err
	}
	return out.(model).chosen, nil
}

type item struct {
	host   *config.Host
	target string
	auth   string
	last   time.Time
	filter string
}

func (i item) FilterValue() string { return i.filter }

type editDoneMsg struct{ err error }

type model struct {
	opts     Options
	cfg      *config.Config
	list     list.Model
	items    []item
	groups   []string
	groupIdx int
	width    int
	height   int
	status   string
	chosen   *config.Host
	delegate *delegate
}

func newModel(o Options) model {
	d := &delegate{}
	l := list.New(nil, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(true)
	l.DisableQuitKeybindings()
	l.KeyMap.CancelWhileFiltering = key.NewBinding()
	l.KeyMap.AcceptWhileFiltering = key.NewBinding()
	l.FilterInput.Prompt = "> "
	l.FilterInput.PromptStyle = accent
	l.FilterInput.Placeholder = "type to filter"
	l.Styles.NoItems = dim
	m := model{opts: o, cfg: o.Config, list: l, groupIdx: -1, delegate: d}
	m.rebuild()
	m.setFilter(o.Filter)
	return m
}

func (m *model) rebuild() {
	hosts := make([]*config.Host, 0, len(m.cfg.Hosts))
	for i := range m.cfg.Hosts {
		hosts = append(hosts, &m.cfg.Hosts[i])
	}
	config.SortHosts(m.cfg, m.opts.State, hosts)
	m.items = m.items[:0]
	ungrouped := false
	for _, h := range hosts {
		r := m.cfg.Resolve(h)
		if h.Group == "" {
			ungrouped = true
		}
		m.items = append(m.items, item{
			host:   h,
			target: fmt.Sprintf("%s@%s:%d", r.User, h.Host, r.Port),
			auth:   r.Auth,
			last:   m.opts.State.Last(h.Name),
			filter: strings.Join([]string{h.Name, h.Host, h.Group, strings.Join(h.Tags, " ")}, " "),
		})
	}
	m.groups = m.groups[:0]
	for _, g := range m.cfg.Groups {
		m.groups = append(m.groups, g.Name)
	}
	if ungrouped {
		m.groups = append(m.groups, "")
	}
	if m.groupIdx >= len(m.groups) {
		m.groupIdx = -1
	}
	m.delegate.cfg = m.cfg
	m.delegate.nameWidth, m.delegate.targetWidth = 0, 0
	for _, it := range m.items {
		m.delegate.nameWidth = max(m.delegate.nameWidth, lipgloss.Width(it.host.Name))
		m.delegate.targetWidth = max(m.delegate.targetWidth, lipgloss.Width(it.target))
	}
	m.applyGroup()
}

func (m *model) applyGroup() {
	var visible []list.Item
	for _, it := range m.items {
		if m.groupIdx >= 0 && !strings.EqualFold(it.host.Group, m.groups[m.groupIdx]) {
			continue
		}
		visible = append(visible, it)
	}
	m.list.SetItems(visible)
	m.setFilter(m.list.FilterValue())
}

func (m *model) setFilter(text string) {
	m.list.SetFilterText(text)
	m.list.SetFilterState(list.Filtering)
}

func (m *model) resize() {
	m.list.SetSize(m.width, max(1, m.height-2))
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case editDoneMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		cfg, err := m.opts.Reload()
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.cfg = cfg
		m.status = ""
		m.rebuild()
		return m, nil
	case tea.KeyMsg:
		m.status = ""
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.list.FilterValue() != "" {
				m.setFilter("")
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if it, ok := m.list.SelectedItem().(item); ok {
				m.chosen = it.host
				return m, tea.Quit
			}
			return m, nil
		case "up", "ctrl+k", "ctrl+p":
			m.list.CursorUp()
			return m, nil
		case "down", "ctrl+j", "ctrl+n":
			m.list.CursorDown()
			return m, nil
		case "pgup":
			m.list.Paginator.PrevPage()
			return m, nil
		case "pgdown":
			m.list.Paginator.NextPage()
			return m, nil
		case "tab", "shift+tab":
			n := len(m.groups) + 1
			step := 1
			if msg.String() == "shift+tab" {
				step = n - 1
			}
			m.groupIdx = (m.groupIdx+1+step)%n - 1
			m.applyGroup()
			return m, nil
		case "ctrl+e":
			if m.opts.Editor == nil {
				return m, nil
			}
			return m, tea.ExecProcess(m.opts.Editor(), func(err error) tea.Msg { return editDoneMsg{err} })
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.list.View())
	b.WriteString("\n")
	b.WriteString(m.footer())
	return b.String()
}

func (m model) header() string {
	scope := "all groups"
	style := dim
	if m.groupIdx >= 0 {
		scope = m.groups[m.groupIdx]
		if scope == "" {
			scope = "no group"
		} else {
			style = lipgloss.NewStyle().Foreground(GroupColor(m.cfg, scope)).Bold(true)
		}
	}
	shown := len(m.list.VisibleItems())
	line := selected.Render("sv") + "  " + style.Render(scope) + dim.Render(fmt.Sprintf("  %d/%d hosts", shown, len(m.items)))
	return ansi.Truncate(line, m.width, "…")
}

func (m model) footer() string {
	if m.status != "" {
		return ansi.Truncate(errStyle.Render(m.status), m.width, "…")
	}
	if len(m.list.VisibleItems()) == 0 && len(m.items) > 0 {
		return dim.Render("no matches · esc clears the filter")
	}
	return ansi.Truncate(dim.Render("enter connect · ↑↓ move · tab group · ctrl+e edit · esc quit"), m.width, "…")
}

type delegate struct {
	cfg         *config.Config
	nameWidth   int
	targetWidth int
}

func (d *delegate) Height() int                         { return 1 }
func (d *delegate) Spacing() int                        { return 0 }
func (d *delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *delegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it, ok := li.(item)
	if !ok {
		return
	}
	cur := index == m.Index()
	cursor := "  "
	name := lipgloss.NewStyle().Width(d.nameWidth).Render(it.host.Name)
	target := dim.Width(d.targetWidth).Render(it.target)
	if cur {
		cursor = accent.Render("▶ ")
		name = selected.Width(d.nameWidth).Render(it.host.Name)
		target = lipgloss.NewStyle().Width(d.targetWidth).Render(it.target)
	}
	parts := []string{cursor + name, target, AuthIcon(it.auth)}
	if it.host.Group != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(GroupColor(d.cfg, it.host.Group)).Render(it.host.Group))
	}
	if len(it.host.Tags) > 0 {
		parts = append(parts, dim.Render(strings.Join(it.host.Tags, ", ")))
	}
	if !it.last.IsZero() {
		parts = append(parts, dim.Render(config.Relative(it.last, time.Now())))
	}
	line := strings.Join(parts, "  ")
	fmt.Fprint(w, ansi.Truncate(line, m.Width(), "…"))
}
