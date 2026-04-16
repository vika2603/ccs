package picker

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/vika2603/ccs/internal/fields"
)

func newTestModel() Model {
	return NewModel(Input{
		Items: []fields.ProfileEntry{
			{Name: "skills", Category: fields.Shared, Kind: fields.KindDir},
			{Name: "CLAUDE.md", Category: fields.Shared, Kind: fields.KindFile},
			{Name: "projects", Category: fields.Isolated, Kind: fields.KindDir},
		},
		SeedSelection:   map[string]bool{"skills": true, "CLAUDE.md": true},
		SeedCredentials: false,
		ProfileName:     "work",
		PresetLabel:     "default",
	})
}

func press(m Model, k string) Model {
	var msg tea.KeyMsg
	switch k {
	case "down":
		msg = tea.KeyMsg(tea.Key{Type: tea.KeyDown})
	case "up":
		msg = tea.KeyMsg(tea.Key{Type: tea.KeyUp})
	case "space":
		msg = tea.KeyMsg(tea.Key{Type: tea.KeySpace})
	case "enter":
		msg = tea.KeyMsg(tea.Key{Type: tea.KeyEnter})
	case "esc":
		msg = tea.KeyMsg(tea.Key{Type: tea.KeyEsc})
	default:
		msg = tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(k)})
	}
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func TestCursorDownAndUp(t *testing.T) {
	m := newTestModel()
	if m.cursor != 0 {
		t.Fatalf("initial cursor=%d", m.cursor)
	}
	m = press(m, "down")
	if m.cursor != 1 {
		t.Fatalf("after down, cursor=%d", m.cursor)
	}
	m = press(m, "j")
	if m.cursor != 2 {
		t.Fatalf("after j, cursor=%d", m.cursor)
	}
	m = press(m, "j")
	if m.cursor != 2 {
		t.Fatalf("cursor must clamp at last index; got %d", m.cursor)
	}
	m = press(m, "up")
	if m.cursor != 1 {
		t.Fatalf("after up, cursor=%d", m.cursor)
	}
}

func TestCredentialsToggle(t *testing.T) {
	m := newTestModel()
	if m.credentials {
		t.Fatalf("seed creds=true unexpectedly")
	}
	m = press(m, "c")
	if !m.credentials {
		t.Fatalf("c should enable creds")
	}
	m = press(m, "c")
	if m.credentials {
		t.Fatalf("c should disable creds")
	}
}

func TestEscCancels(t *testing.T) {
	m := newTestModel()
	m = press(m, "esc")
	if !m.done || !m.result.Cancelled {
		t.Fatalf("Esc must set done + Cancelled; done=%v cancelled=%v", m.done, m.result.Cancelled)
	}
}

func TestQuitCancels(t *testing.T) {
	m := newTestModel()
	m = press(m, "q")
	if !m.done || !m.result.Cancelled {
		t.Fatalf("q must cancel")
	}
}

func TestEnterConfirms(t *testing.T) {
	m := newTestModel()
	m = press(m, "enter")
	if !m.done || m.result.Cancelled {
		t.Fatalf("Enter must set done and NOT cancelled; done=%v cancelled=%v", m.done, m.result.Cancelled)
	}
	got := map[string]bool{}
	for _, n := range m.result.Names {
		got[n] = true
	}
	want := map[string]bool{"skills": true, "CLAUDE.md": true}
	if len(got) != len(want) {
		t.Fatalf("Enter names=%v; want %v", m.result.Names, want)
	}
	for n := range want {
		if !got[n] {
			t.Fatalf("missing %s in result names", n)
		}
	}
}

func TestCtrlCInterrupts(t *testing.T) {
	m := newTestModel()
	msg := tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC})
	nm, _ := m.Update(msg)
	m = nm.(Model)
	if !m.done || !m.interrupted {
		t.Fatalf("Ctrl-C must set done and interrupted; done=%v interrupted=%v", m.done, m.interrupted)
	}
}

func TestHelpToggles(t *testing.T) {
	m := newTestModel()
	m = press(m, "?")
	if !m.helpOpen {
		t.Fatalf("? should open help")
	}
	m = press(m, "?")
	if m.helpOpen {
		t.Fatalf("? should close help")
	}
}

func TestResetConfirmAccept(t *testing.T) {
	m := newTestModel()
	m = press(m, "space")
	if m.selection["skills"] {
		t.Fatalf("space should have unchecked skills")
	}
	m = press(m, "c")
	if !m.credentials {
		t.Fatalf("c should have enabled creds")
	}
	m = press(m, "R")
	if !m.confirmReset {
		t.Fatalf("R should open confirm modal")
	}
	m = press(m, "y")
	if m.confirmReset {
		t.Fatalf("y should close the modal")
	}
	if !m.selection["skills"] {
		t.Fatalf("reset should restore skills=true")
	}
	if m.credentials {
		t.Fatalf("reset should restore creds=false")
	}
}

func TestResetConfirmReject(t *testing.T) {
	m := newTestModel()
	m = press(m, "space")
	m = press(m, "R")
	m = press(m, "n")
	if m.confirmReset {
		t.Fatalf("n should close the modal")
	}
	if m.selection["skills"] {
		t.Fatalf("rejecting reset must NOT re-seed; skills should still be false")
	}
}

func TestGotoTopAndBottom(t *testing.T) {
	m := newTestModel()
	m = press(m, "G")
	if m.cursor != 2 {
		t.Fatalf("G should jump to last index; cursor=%d", m.cursor)
	}
	m = press(m, "g")
	if m.cursor != 0 {
		t.Fatalf("g should jump to first index; cursor=%d", m.cursor)
	}
}

func TestRunPickerConfirmsViaTeatest(t *testing.T) {
	in := Input{
		Items: []fields.ProfileEntry{
			{Name: "skills", Category: fields.Shared, Kind: fields.KindDir},
			{Name: "projects", Category: fields.Isolated, Kind: fields.KindDir},
		},
		SeedSelection:   map[string]bool{"skills": true},
		SeedCredentials: false,
		ProfileName:     "work",
		PresetLabel:     "default",
	}

	tm := teatest.NewTestModel(t, NewModel(in),
		teatest.WithInitialTermSize(80, 20))
	tm.Send(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	fm, ok := tm.FinalModel(t).(Model)
	if !ok {
		t.Fatalf("final model wrong type")
	}
	if !fm.done || fm.result.Cancelled {
		t.Fatalf("expected done + not cancelled, got done=%v cancelled=%v",
			fm.done, fm.result.Cancelled)
	}
	if !strings.Contains(strings.Join(fm.result.Names, ","), "skills") {
		t.Fatalf("skills not in result names: %v", fm.result.Names)
	}
}

func TestViewContainsEntryAndPreset(t *testing.T) {
	m := newTestModel()
	wm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = wm.(Model)
	out := m.View()
	for _, s := range []string{"work", "default", "skills", "CLAUDE.md", "projects", "◉", "○"} {
		if !strings.Contains(out, s) {
			t.Errorf("View missing %q; got:\n%s", s, out)
		}
	}
}

func TestViewShowsConfirmModal(t *testing.T) {
	m := newTestModel()
	wm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = wm.(Model)
	m = press(m, "R")
	out := m.View()
	if !strings.Contains(out, "Reset to preset") || !strings.Contains(out, "y/N") {
		t.Errorf("View missing confirm-modal text; got:\n%s", out)
	}
}

func TestCursorScrollsViewport(t *testing.T) {
	items := make([]fields.ProfileEntry, 20)
	for i := range items {
		items[i] = fields.ProfileEntry{
			Name: string(rune('a' + i)), Category: fields.Shared, Kind: fields.KindFile,
		}
	}
	m := NewModel(Input{Items: items, SeedSelection: map[string]bool{}})
	wm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = wm.(Model)
	if m.viewportHeight <= 0 {
		t.Fatalf("viewportHeight not set; got %d", m.viewportHeight)
	}
	for i := 0; i < 10; i++ {
		m = press(m, "down")
	}
	if m.viewportOffset == 0 {
		t.Fatalf("viewport should have scrolled; offset=0 cursor=%d height=%d",
			m.cursor, m.viewportHeight)
	}
	if m.cursor < m.viewportOffset || m.cursor >= m.viewportOffset+m.viewportHeight {
		t.Fatalf("cursor %d outside viewport [%d, %d)",
			m.cursor, m.viewportOffset, m.viewportOffset+m.viewportHeight)
	}
}

func TestSpaceTogglesCurrentRow(t *testing.T) {
	m := newTestModel()
	if !m.selection["skills"] {
		t.Fatalf("seed expected skills=true")
	}
	m = press(m, "space")
	if m.selection["skills"] {
		t.Fatalf("space should have un-checked skills")
	}
	m = press(m, "down")
	m = press(m, "down")
	m = press(m, "space")
	if !m.selection["projects"] {
		t.Fatalf("space should have checked projects")
	}
}
