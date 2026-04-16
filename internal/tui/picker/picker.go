package picker

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vika2603/ccs/internal/fields"
)

var (
	headerStyle    = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	activeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	checkedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	uncheckedStyle = lipgloss.NewStyle().Faint(true)
	unknownStyle  = lipgloss.NewStyle().Faint(true)
	footerStyle   = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	modalStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			Foreground(lipgloss.Color("3"))
)

var ErrSIGINT = errors.New("picker: interrupted")

type Input struct {
	Items           []fields.ProfileEntry
	SeedSelection   map[string]bool
	SeedCredentials bool
	ProfileName     string
	PresetLabel     string
}

type Result struct {
	Names       []string
	Credentials bool
	Cancelled   bool
}

type Model struct {
	input          Input
	selection      map[string]bool
	credentials    bool
	cursor         int
	viewportOffset int
	viewportHeight int
	confirmReset   bool
	helpOpen       bool
	width          int
	height         int
	result         Result
	done           bool
	interrupted    bool
}

func NewModel(in Input) Model {
	sel := make(map[string]bool, len(in.SeedSelection))
	for k, v := range in.SeedSelection {
		sel[k] = v
	}
	return Model{
		input:       in,
		selection:   sel,
		credentials: in.SeedCredentials,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.confirmReset {
			return m.handleConfirmReset(msg), nil
		}
		switch msg.Type {
		case tea.KeyEnter:
			return m.commit(), tea.Quit
		case tea.KeyEsc:
			return m.cancel(), tea.Quit
		case tea.KeyCtrlC:
			return m.interrupt(), tea.Quit
		case tea.KeyDown:
			m.cursor = clamp(m.cursor+1, 0, len(m.input.Items)-1)
			m.ensureCursorVisible()
		case tea.KeyUp:
			m.cursor = clamp(m.cursor-1, 0, len(m.input.Items)-1)
			m.ensureCursorVisible()
		case tea.KeySpace:
			m.toggleCurrent()
		case tea.KeyHome:
			m.cursor = 0
			m.ensureCursorVisible()
		case tea.KeyEnd:
			if len(m.input.Items) > 0 {
				m.cursor = len(m.input.Items) - 1
			}
			m.ensureCursorVisible()
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "j":
				m.cursor = clamp(m.cursor+1, 0, len(m.input.Items)-1)
				m.ensureCursorVisible()
			case "k":
				m.cursor = clamp(m.cursor-1, 0, len(m.input.Items)-1)
				m.ensureCursorVisible()
			case "g":
				m.cursor = 0
				m.ensureCursorVisible()
			case "G":
				if len(m.input.Items) > 0 {
					m.cursor = len(m.input.Items) - 1
				}
				m.ensureCursorVisible()
			case "c":
				m.credentials = !m.credentials
			case "q":
				return m.cancel(), tea.Quit
			case "?":
				m.helpOpen = !m.helpOpen
			case "R":
				m.confirmReset = true
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if msg.Height-6 > 1 {
			m.viewportHeight = msg.Height - 6
		} else {
			m.viewportHeight = 1
		}
	}
	return m, nil
}

func (m *Model) toggleCurrent() {
	if len(m.input.Items) == 0 {
		return
	}
	name := m.input.Items[m.cursor].Name
	m.selection[name] = !m.selection[name]
}

func (m Model) commit() Model {
	names := make([]string, 0, len(m.selection))
	for _, it := range m.input.Items {
		if m.selection[it.Name] {
			names = append(names, it.Name)
		}
	}
	m.result = Result{Names: names, Credentials: m.credentials}
	m.done = true
	return m
}

func (m Model) cancel() Model {
	m.result = Result{Cancelled: true}
	m.done = true
	return m
}

func (m Model) interrupt() Model {
	m.result = Result{Cancelled: true}
	m.done = true
	m.interrupted = true
	return m
}

func (m Model) handleConfirmReset(msg tea.KeyMsg) Model {
	if msg.Type == tea.KeyRunes {
		switch string(msg.Runes) {
		case "y", "Y":
			sel := map[string]bool{}
			for k, v := range m.input.SeedSelection {
				sel[k] = v
			}
			m.selection = sel
			m.credentials = m.input.SeedCredentials
		}
	}
	m.confirmReset = false
	return m
}

func (m *Model) ensureCursorVisible() {
	if m.viewportHeight <= 0 {
		return
	}
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	} else if m.cursor >= m.viewportOffset+m.viewportHeight {
		m.viewportOffset = m.cursor - m.viewportHeight + 1
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m Model) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder
	header := fmt.Sprintf("Export profile %q   Preset: %s", m.input.ProfileName, m.input.PresetLabel)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", clamp(m.width, 20, 120)))
	b.WriteString("\n")

	if m.helpOpen {
		b.WriteString(modalStyle.Render(helpText()))
		b.WriteString("\n")
	}

	start := m.viewportOffset
	end := start + m.viewportHeight
	if m.viewportHeight == 0 {
		end = len(m.input.Items)
	}
	if end > len(m.input.Items) {
		end = len(m.input.Items)
	}

	for i := start; i < end; i++ {
		it := m.input.Items[i]
		icon := uncheckedStyle.Render("○")
		if m.selection[it.Name] {
			icon = checkedStyle.Render("◉")
		}
		prefix := "  "
		if i == m.cursor {
			prefix = activeStyle.Render("› ")
		}
		row := fmt.Sprintf("%s%s %-20s %-5s %s", prefix, icon, truncate(it.Name, 20), kindLabel(it.Kind), sizeLabel(it))
		if it.IsUnknown {
			row += "  " + unknownStyle.Render("unknown")
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	credIcon := uncheckedStyle.Render("○")
	if m.credentials {
		credIcon = checkedStyle.Render("◉")
	}
	b.WriteString(fmt.Sprintf("OAuth credentials: %s\n", credIcon))
	b.WriteString(footerStyle.Render("Enter confirm   Esc cancel   Space toggle   R reset   c creds   ? help"))

	if m.confirmReset {
		b.WriteString("\n")
		b.WriteString(modalStyle.Render("Reset to preset? y/N"))
	}
	return b.String()
}

func categoryLabel(c fields.Category) string {
	switch c {
	case fields.Shared:
		return "Shared"
	default:
		return "Isolated"
	}
}

func kindLabel(k fields.Kind) string {
	if k == fields.KindDir {
		return "dir"
	}
	return "file"
}

func sizeLabel(it fields.ProfileEntry) string {
	if it.Kind == fields.KindDir {
		return fmt.Sprintf("%d files", it.FileCount)
	}
	return humanBytes(it.Size)
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	}
	return fmt.Sprintf("%.1f GB", float64(n)/1024/1024/1024)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "..."
}

func helpText() string {
	return "up/k up  down/j down  space toggle  c creds  R reset  ? help  Enter confirm  Esc cancel"
}

func RunPicker(in Input) (res Result, err error) {
	m := NewModel(in)
	p := tea.NewProgram(m, tea.WithAltScreen())
	defer func() {
		if rec := recover(); rec != nil {
			p.ReleaseTerminal()
			panic(rec)
		}
	}()
	final, runErr := p.Run()
	if runErr != nil {
		return Result{}, runErr
	}
	fm, ok := final.(Model)
	if !ok {
		return Result{}, errors.New("picker: unexpected final model")
	}
	if fm.interrupted {
		return Result{}, ErrSIGINT
	}
	return fm.result, nil
}
