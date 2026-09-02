package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kianurivzzz/save-serve-cli/internal/config"
)

func testModel(t *testing.T, filter string) model {
	t.Helper()
	cfg := &config.Config{
		Defaults: config.Defaults{User: "root", Port: 22},
		Groups:   []config.Group{{Name: "work", Color: "orange"}, {Name: "home"}},
		Hosts: []config.Host{
			{Name: "coolify", Host: "1.2.3.4", Group: "home", Auth: config.AuthAgent, Tags: []string{"docker"}},
			{Name: "CashCow", Host: "cashcow.example.com", Group: "work", Auth: config.AuthPassword},
			{Name: "crown", Host: "10.0.0.1", Group: "work", Auth: config.AuthAgent},
			{Name: "lonely", Host: "10.0.0.2", Auth: config.AuthAgent},
		},
	}
	st := &config.State{LastUsed: map[string]time.Time{"crown": time.Now()}}
	m := newModel(Options{Config: cfg, State: st, Filter: filter})
	m.list.FilterInput.Cursor.SetMode(cursor.CursorStatic)
	m = drive(t, m, tea.WindowSizeMsg{Width: 100, Height: 20})
	return m
}

func drive(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(model)
	return runCmd(t, m, cmd)
}

func runCmd(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			m = runCmd(t, m, c)
		}
		return m
	case list.FilterMatchesMsg:
		return drive(t, m, msg)
	default:
		return m
	}
}

func typeText(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = drive(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func press(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }

func names(m model) []string {
	var out []string
	for _, it := range m.list.VisibleItems() {
		out = append(out, it.(item).host.Name)
	}
	return out
}

func TestOrderAndView(t *testing.T) {
	m := testModel(t, "")
	if got := strings.Join(names(m), " "); got != "crown CashCow coolify lonely" {
		t.Errorf("order: %s", got)
	}
	v := m.View()
	for _, want := range []string{"crown", "CashCow", "coolify", "lonely", "root@1.2.3.4:22", "🔐", "🔒", "docker", "all groups", "4/4 hosts", "just now"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
}

func TestTypeToFilterAndEnter(t *testing.T) {
	m := testModel(t, "")
	m = typeText(t, m, "cash")
	if got := names(m); len(got) != 1 || got[0] != "CashCow" {
		t.Errorf("filter cash: %v", got)
	}
	next, cmd := m.Update(press(tea.KeyEnter))
	m = next.(model)
	if m.chosen == nil || m.chosen.Name != "CashCow" {
		t.Errorf("chosen = %v", m.chosen)
	}
	if cmd == nil {
		t.Fatal("enter must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("enter must return tea.Quit")
	}
}

func TestPrefilledFilter(t *testing.T) {
	m := testModel(t, "lonely")
	got := names(m)
	if len(got) != 1 || got[0] != "lonely" {
		t.Errorf("prefilled filter: %v", got)
	}
	if m.list.FilterValue() != "lonely" {
		t.Errorf("filter text = %q", m.list.FilterValue())
	}
}

func TestEscClearsThenQuits(t *testing.T) {
	m := typeText(t, testModel(t, ""), "zzz")
	if len(names(m)) != 0 || !strings.Contains(m.View(), "no matches") {
		t.Errorf("no-match state: %v\n%s", names(m), m.View())
	}
	next, cmd := m.Update(press(tea.KeyEsc))
	m = next.(model)
	if cmd != nil || m.list.FilterValue() != "" || len(names(m)) != 4 {
		t.Errorf("first esc must clear filter: cmd=%v filter=%q n=%d", cmd, m.list.FilterValue(), len(names(m)))
	}
	next, cmd = m.Update(press(tea.KeyEsc))
	m = next.(model)
	if cmd == nil || m.chosen != nil {
		t.Fatal("second esc must quit without a host")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc must return tea.Quit")
	}
}

func TestNavigation(t *testing.T) {
	m := testModel(t, "")
	m = drive(t, m, press(tea.KeyDown))
	m = drive(t, m, press(tea.KeyCtrlJ))
	if it := m.list.SelectedItem().(item); it.host.Name != "coolify" {
		t.Errorf("after two downs: %s", it.host.Name)
	}
	m = drive(t, m, press(tea.KeyCtrlK))
	if it := m.list.SelectedItem().(item); it.host.Name != "CashCow" {
		t.Errorf("after up: %s", it.host.Name)
	}
	m = typeText(t, m, "l")
	if it := m.list.SelectedItem().(item); it.host.Name != "coolify" && it.host.Name != "lonely" {
		t.Errorf("typing must reset cursor to first match, got %s", it.host.Name)
	}
}

func TestTabCyclesGroups(t *testing.T) {
	m := testModel(t, "")
	m = drive(t, m, press(tea.KeyTab))
	if got := strings.Join(names(m), " "); got != "crown CashCow" || !strings.Contains(m.View(), "work") {
		t.Errorf("group work: %s", got)
	}
	m = drive(t, m, press(tea.KeyTab))
	if got := strings.Join(names(m), " "); got != "coolify" {
		t.Errorf("group home: %s", got)
	}
	m = drive(t, m, press(tea.KeyTab))
	if got := strings.Join(names(m), " "); got != "lonely" || !strings.Contains(m.View(), "no group") {
		t.Errorf("no group: %s", got)
	}
	m = drive(t, m, press(tea.KeyTab))
	if len(names(m)) != 4 {
		t.Errorf("back to all: %v", names(m))
	}
	m = drive(t, m, press(tea.KeyShiftTab))
	if got := strings.Join(names(m), " "); got != "lonely" {
		t.Errorf("shift+tab wraps backwards: %s", got)
	}
	m = typeText(t, m, "lonely")
	m = drive(t, m, press(tea.KeyTab))
	if got := names(m); len(got) != 1 || got[0] != "lonely" || m.list.FilterValue() != "lonely" {
		t.Errorf("filter must survive group switch: %v %q", got, m.list.FilterValue())
	}
	m = drive(t, m, press(tea.KeyTab))
	if len(names(m)) != 0 || !strings.Contains(m.View(), "no matches") {
		t.Errorf("group work with filter lonely: %v", names(m))
	}
}

func TestEditReload(t *testing.T) {
	m := testModel(t, "")
	fresh := &config.Config{Hosts: []config.Host{{Name: "only", Host: "h"}}}
	m.opts.Reload = func() (*config.Config, error) { return fresh, nil }
	m = drive(t, m, editDoneMsg{})
	if got := names(m); len(got) != 1 || got[0] != "only" {
		t.Errorf("after reload: %v", got)
	}
	m.opts.Reload = func() (*config.Config, error) { return nil, errFake }
	m = drive(t, m, editDoneMsg{})
	if len(names(m)) != 1 || !strings.Contains(m.View(), "fake") {
		t.Errorf("reload error must keep old hosts and show status:\n%s", m.View())
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "fake yaml error at line 3" }

var errFake = fakeErr{}
