package tui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kamskr/fleet/internal/app"
	"github.com/kamskr/fleet/internal/session"
)

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeRename
	modeChangeDir
	modeConfirmKill
)

type loadedMsg []session.Session
type errMsg error
type dirSelectedMsg string
type attachReadyMsg string

const (
	ctpBase     = lipgloss.Color("#1e1e2e")
	ctpSurface0 = lipgloss.Color("#313244")
	ctpSurface1 = lipgloss.Color("#45475a")
	ctpText     = lipgloss.Color("#cdd6f4")
	ctpSubtext0 = lipgloss.Color("#a6adc8")
	ctpOverlay0 = lipgloss.Color("#6c7086")
	ctpBlue     = lipgloss.Color("#89b4fa")
	ctpMauve    = lipgloss.Color("#cba6f7")
	ctpPink     = lipgloss.Color("#f5c2e7")
	ctpRed      = lipgloss.Color("#f38ba8")
	ctpGreen    = lipgloss.Color("#a6e3a1")
)

type model struct {
	app      app.App
	sessions []session.Session
	cursor   int
	grouped  bool
	filter   string
	mode     mode
	input    textinput.Model
	err      string
	width    int
	height   int
}

func Run(a app.App) error {
	m := newModel(a)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return err
	}
	return nil
}

func newModel(a app.App) model {
	in := textinput.New()
	in.Prompt = "> "
	in.PromptStyle = lipgloss.NewStyle().Foreground(ctpMauve)
	in.TextStyle = lipgloss.NewStyle().Foreground(ctpText)
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(ctpOverlay0)
	in.CharLimit = 512
	return model{app: a, input: in}
}

func (m model) Init() tea.Cmd { return m.load }

func (m model) load() tea.Msg {
	s, err := m.app.Sessions(context.Background())
	if err != nil {
		return errMsg(err)
	}
	return loadedMsg(s)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.sessions = []session.Session(msg)
		if m.cursor >= len(m.visible()) {
			m.cursor = max(0, len(m.visible())-1)
		}
	case errMsg:
		m.err = msg.Error()
	case dirSelectedMsg:
		if string(msg) != "" && m.mode == modeChangeDir {
			m.input.SetValue(string(msg))
			m.input.CursorEnd()
		}
	case attachReadyMsg:
		name := string(msg)
		return m, tea.ExecProcess(m.app.Tmux.AttachCommand(name), func(err error) tea.Msg {
			if err != nil {
				return errMsg(err)
			}
			return m.load()
		})
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.mode != modeNormal {
			return m.updateInput(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.visible())-1 {
				m.cursor++
			}
		case "enter":
			if s, ok := m.selected(); ok && s.Status == session.StatusRunning {
				return m, m.prepareAttach(s.TmuxSessionName)
			}
		case "n":
			return m, m.createAndAttach()
		case "/":
			m.startInput(modeFilter, "Filter", m.filter)
		case "g":
			m.grouped = !m.grouped
		case "p":
			if s, ok := m.selected(); ok {
				return m, m.wrap(func() error { return m.app.TogglePin(context.Background(), s.ID) })
			}
		case "x":
			if _, ok := m.selected(); ok {
				m.mode = modeConfirmKill
			}
		case "r":
			if s, ok := m.selected(); ok {
				m.startInput(modeRename, "Rename", s.DisplayName)
			}
		case "d":
			if s, ok := m.selected(); ok {
				m.startInput(modeChangeDir, "Directory", s.Directory)
			}
		}
	}
	return m, nil
}

func (m model) prepareAttach(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.app.Tmux.SetupFleetChrome(context.Background(), name); err != nil {
			return errMsg(err)
		}
		return attachReadyMsg(name)
	}
}

func (m model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeConfirmKill {
		switch msg.String() {
		case "y", "Y":
			if s, ok := m.selected(); ok {
				m.mode = modeNormal
				return m, m.wrap(func() error { return m.app.Kill(context.Background(), s.ID) })
			}
		case "n", "esc", "q":
			m.mode = modeNormal
		}
		return m, nil
	}
	if msg.String() == "esc" {
		m.mode = modeNormal
		return m, nil
	}
	if msg.String() == "ctrl+f" && m.mode == modeChangeDir {
		return m, fzfDirCmd(m.input.Value())
	}
	if msg.String() == "enter" {
		v := strings.TrimSpace(m.input.Value())
		switch m.mode {
		case modeFilter:
			m.filter = v
			m.cursor = 0
			m.mode = modeNormal
		case modeRename:
			if s, ok := m.selected(); ok {
				m.mode = modeNormal
				return m, m.wrap(func() error { return m.app.Rename(context.Background(), s.ID, v) })
			}
		case modeChangeDir:
			if s, ok := m.selected(); ok {
				m.mode = modeNormal
				return m, m.wrap(func() error { return m.app.ChangeDir(context.Background(), s.ID, v) })
			}
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) createAndAttach() tea.Cmd {
	return func() tea.Msg {
		s, err := m.app.Create(context.Background(), "", "", "")
		if err != nil {
			return errMsg(err)
		}
		return attachReadyMsg(s.TmuxSessionName)
	}
}

func (m *model) startInput(md mode, placeholder, value string) {
	m.mode = md
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.input.Focus()
}

func (m model) wrap(fn func() error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(); err != nil {
			return errMsg(err)
		}
		s, err := m.app.Sessions(context.Background())
		if err != nil {
			return errMsg(err)
		}
		return loadedMsg(s)
	}
}

func fzfDirCmd(initial string) tea.Cmd {
	if _, err := exec.LookPath("fzf"); err != nil {
		return func() tea.Msg { return errMsg(fmt.Errorf("fzf not found; install fzf or type the directory manually")) }
	}
	input, err := os.CreateTemp("", "fleet-dirs-*.txt")
	if err != nil {
		return func() tea.Msg { return errMsg(err) }
	}
	output, err := os.CreateTemp("", "fleet-dir-choice-*.txt")
	if err != nil {
		_ = os.Remove(input.Name())
		return func() tea.Msg { return errMsg(err) }
	}
	choices := directoryCandidates(initial)
	_, _ = input.WriteString(strings.Join(choices, "\n"))
	_ = input.Close()
	_ = output.Close()
	cmd := exec.Command("fzf", "--prompt=Fleet dir> ", "--height=45%", "--border", "--reverse", "--query", initial)
	in, _ := os.Open(input.Name())
	out, _ := os.Create(output.Name())
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		_ = in.Close()
		_ = out.Close()
		defer os.Remove(input.Name())
		defer os.Remove(output.Name())
		if err != nil {
			return dirSelectedMsg("")
		}
		b, readErr := os.ReadFile(output.Name())
		if readErr != nil {
			return errMsg(readErr)
		}
		return dirSelectedMsg(strings.TrimSpace(string(b)))
	})
}

func directoryCandidates(initial string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(p string) {
		if p == "" {
			return
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	add(initial)
	if cwd, err := os.Getwd(); err == nil {
		add(cwd)
		walkDirs(cwd, 3, add)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(home)
		add(filepath.Join(home, "Files"))
		walkDirs(filepath.Join(home, "Files"), 3, add)
	}
	sort.Strings(out)
	return out
}

func walkDirs(root string, maxDepth int, add func(string)) {
	root = filepath.Clean(root)
	rootDepth := strings.Count(root, string(os.PathSeparator))
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		base := d.Name()
		if base == ".git" || base == "node_modules" || base == ".cache" || base == "Library" {
			return filepath.SkipDir
		}
		depth := strings.Count(filepath.Clean(path), string(os.PathSeparator)) - rootDepth
		if depth > maxDepth {
			return filepath.SkipDir
		}
		add(path)
		return nil
	})
}

func (m model) View() string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(ctpBlue).Render("Fleet")
	body := title + "  " + muted("AI coding sessions on tmux -L fleet") + "\n\n"
	if m.err != "" {
		body += lipgloss.NewStyle().Foreground(ctpRed).Render(m.err) + "\n"
	}
	if m.mode != modeNormal {
		if m.mode == modeConfirmKill {
			if s, ok := m.selected(); ok && s.Status != session.StatusRunning {
				return m.overlay(body+"Remove dead session from Fleet? y/N\n", width)
			}
			return m.overlay(body+"Kill selected session and remove it from Fleet? y/N\n", width)
		}
		extra := "Esc cancels"
		if m.mode == modeChangeDir {
			extra = "Ctrl+F opens fzf • Esc cancels"
		}
		return m.overlay(body+promptFor(m.mode)+"\n"+m.input.View()+"\n\n"+muted(extra), width)
	}
	vis := m.visible()
	if len(vis) == 0 {
		body += muted("No sessions. Press n to create one.") + "\n"
	} else {
		body += m.renderList(vis)
	}
	body += "\n" + m.renderDetails() + "\n"
	body += muted("n new • enter attach • p pin • d dir • r rename • x kill/remove • / filter • g group • q quit")
	return m.overlay(body, width)
}

func (m model) renderList(vis []session.Session) string {
	var b strings.Builder
	lastGroup := ""
	if m.grouped {
		for i, s := range vis {
			group := s.Directory
			if s.Pinned {
				group = "Pinned"
			}
			if group != lastGroup {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				lastGroup = group
				b.WriteString(groupHeader(group) + "\n\n")
			}
			b.WriteString(m.renderGroupedSessionRow(i, s) + "\n")
		}
		return b.String()
	}
	for i, s := range vis {
		b.WriteString(m.renderSessionRow(i, s) + "\n")
	}
	return b.String()
}

func (m model) renderSessionRow(i int, s session.Session) string {
	cursor := "  "
	if i == m.cursor {
		cursor = "▸ "
	}
	pin := " "
	if s.Pinned {
		pin = "★"
	}
	line := fmt.Sprintf("%s%s%s    %s", cursor, pin, trim(s.DisplayName, 24), trim(responsePreview(s), 78))
	if i == m.cursor {
		line = lipgloss.NewStyle().Foreground(ctpPink).Bold(true).Render(line)
	}
	return line
}

func (m model) renderGroupedSessionRow(i int, s session.Session) string {
	return m.renderSessionRow(i, s)
}

func groupHeader(s string) string {
	return lipgloss.NewStyle().Foreground(ctpBlue).Bold(true).Render("▾ " + s)
}

func (m model) overlay(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	panelWidth := min(max(72, width-8), 118)
	for i := range lines {
		lines[i] = pad(lines[i], panelWidth-4)
	}
	panel := lipgloss.NewStyle().
		Width(panelWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ctpMauve).
		Background(ctpBase).
		Foreground(ctpText).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.NewStyle().Margin(1, 0, 0, 2).Render(panel)
}

func (m model) renderDetails() string {
	s, ok := m.selected()
	if !ok {
		return ""
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(ctpSurface1).Padding(0, 1).Render(
		fmt.Sprintf("%s\nid: %s\ntmux: %s\nstatus: %s\nactivity: %s\ndir: %s\ncmd: %s\nlast: %s\npinned: %t\ncreated: %s", trim(s.DisplayName, 48), s.ID, trim(s.TmuxSessionName, 56), s.Status, activityLabel(s), trim(s.Directory, 64), trim(commandLabel(s), 64), trim(responsePreview(s), 72), s.Pinned, s.CreatedAt.Format("2006-01-02 15:04")))
}

func (m model) visible() []session.Session {
	out := []session.Session{}
	q := strings.ToLower(strings.TrimSpace(m.filter))
	for _, s := range m.sessions {
		if q == "" || strings.Contains(strings.ToLower(s.DisplayName+" "+s.Directory+" "+s.Command), q) {
			out = append(out, s)
		}
	}
	session.Sort(out)
	if m.grouped {
		pinned := []session.Session{}
		other := []session.Session{}
		for _, s := range out {
			if s.Pinned {
				pinned = append(pinned, s)
			} else {
				other = append(other, s)
			}
		}
		sort.SliceStable(other, func(i, j int) bool {
			if other[i].Directory != other[j].Directory {
				return other[i].Directory < other[j].Directory
			}
			return other[i].DisplayName < other[j].DisplayName
		})
		out = append(pinned, other...)
	}
	return out
}

func (m model) selected() (session.Session, bool) {
	v := m.visible()
	if m.cursor < 0 || m.cursor >= len(v) {
		return session.Session{}, false
	}
	return v[m.cursor], true
}

func promptFor(md mode) string {
	switch md {
	case modeFilter:
		return "Filter sessions"
	case modeRename:
		return "Rename session"
	case modeChangeDir:
		return "Change saved directory (new tmux sessions only)"
	default:
		return "Input"
	}
}
func muted(s string) string { return lipgloss.NewStyle().Foreground(ctpSubtext0).Render(s) }
func commandLabel(s session.Session) string {
	if strings.TrimSpace(s.Command) == "" {
		return "default shell"
	}
	return s.Command
}
func responsePreview(s session.Session) string {
	if s.Status != session.StatusRunning {
		if s.Status == session.StatusDead {
			return "stopped"
		}
		return string(s.Status)
	}
	activity := activityLabel(s)
	last := strings.TrimSpace(s.LastResponse)
	if last == "" || last == "waiting for output" || looksLikeShellPrompt(last) {
		return "waiting for output"
	}
	return activity + ": " + last
}

func looksLikeShellPrompt(s string) bool {
	fields := strings.Fields(s)
	if len(fields) > 6 {
		return false
	}
	return strings.HasPrefix(s, "~/") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "$ ") || strings.HasPrefix(s, "> ") || strings.HasPrefix(s, "❯ ")
}

func activityLabel(s session.Session) string {
	if s.Status != session.StatusRunning {
		return "stopped"
	}
	if s.LastActivityAt == nil {
		return "waiting"
	}
	d := time.Since(*s.LastActivityAt)
	if d < 15*time.Second {
		return "working"
	}
	if d < time.Minute {
		return "idle " + fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return "idle " + fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return "idle " + fmt.Sprintf("%dh", int(d.Hours()))
}
func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
func pad(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
