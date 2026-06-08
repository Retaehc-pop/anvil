package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Retaehc-pop/anvil/internal/parser"
	"github.com/Retaehc-pop/anvil/internal/runner"
	"github.com/Retaehc-pop/anvil/internal/tui/styles"
)

// RunDoneMsg is sent when the ansible subprocess finishes.
type RunDoneMsg struct{ Err error }

// taskStatus tracks per-task host counters.
type taskStatus struct {
	name        string
	ok          int
	changed     int
	failed      int
	skipped     int
	inProgress  int
	done        bool
}

func (t taskStatus) icon() string {
	if t.failed > 0 && t.ok == 0 && t.changed == 0 {
		return styles.IconFailed
	}
	if t.failed > 0 {
		return styles.IconPartial
	}
	if t.done {
		return styles.IconOK
	}
	return styles.IconRunning
}

// Run is the active-playbook screen.
type Run struct {
	opts      RunOptionsMsg
	width     int
	height    int
	focus     int // 0=task list, 1=progress, 2=output
	showHelp  bool

	tasks       []taskStatus
	taskIdx     int
	currentTask int

	currentPlay string
	elapsed     time.Duration
	startTime   time.Time

	// output lines (all + stderr)
	outputLines  []string
	stderrLines  []string
	outputFilter string

	outputVP  viewport.Model
	stderrVP  viewport.Model

	// totals
	totalOK      int
	totalChanged int
	totalFailed  int
	totalSkipped int
}

// tickMsg drives the elapsed timer.
type tickMsg struct{}

func NewRun(opts RunOptionsMsg) Run {
	ovp := viewport.New(0, 0)
	svp := viewport.New(0, 0)
	return Run{
		opts:      opts,
		startTime: time.Now(),
		outputVP:  ovp,
		stderrVP:  svp,
	}
}

func (r Run) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg { return tickMsg{} })
}

func (r Run) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		r.resizeViewports()

	case tickMsg:
		r.elapsed = time.Since(r.startTime)
		cmds = append(cmds, tickCmd())

	case runner.LineMsg:
		r = r.handleEvent(msg)
		r.refreshOutputVP()

	case RunDoneMsg:
		// Mark current task done
		if r.currentTask < len(r.tasks) {
			r.tasks[r.currentTask].done = true
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return r, func() tea.Msg { return RunDoneMsg{} }
		case "?":
			r.showHelp = !r.showHelp
		case "q", "ctrl+c":
			return r, tea.Quit
		case "tab":
			r.focus = (r.focus + 1) % 2
		case "up", "k":
			if r.focus == 0 && r.taskIdx > 0 {
				r.taskIdx--
			}
		case "down", "j":
			if r.focus == 0 && r.taskIdx < len(r.tasks)-1 {
				r.taskIdx++
			}
		default:
			var vpCmd tea.Cmd
			r.outputVP, vpCmd = r.outputVP.Update(msg)
			cmds = append(cmds, vpCmd)
		}
	}

	return r, tea.Batch(cmds...)
}

func (r Run) handleEvent(msg runner.LineMsg) Run {
	if msg.Stderr {
		r.stderrLines = append(r.stderrLines, msg.Event.Raw)
		return r
	}
	ev := msg.Event
	switch ev.Type {
	case parser.EventPlayStart:
		r.currentPlay = ev.Play

	case parser.EventTaskStart:
		if r.currentTask < len(r.tasks) {
			r.tasks[r.currentTask].done = true
		}
		r.tasks = append(r.tasks, taskStatus{name: ev.Task})
		r.currentTask = len(r.tasks) - 1

	case parser.EventRunnerOK:
		if r.currentTask < len(r.tasks) {
			r.tasks[r.currentTask].ok++
		}
		r.totalOK++
		r.appendOutput(fmt.Sprintf("ok: [%s]", ev.Host))

	case parser.EventRunnerChanged:
		if r.currentTask < len(r.tasks) {
			r.tasks[r.currentTask].changed++
		}
		r.totalChanged++
		r.appendOutput(fmt.Sprintf("changed: [%s]", ev.Host))

	case parser.EventRunnerFailed:
		if r.currentTask < len(r.tasks) {
			r.tasks[r.currentTask].failed++
		}
		r.totalFailed++
		r.appendOutput(fmt.Sprintf("failed: [%s] %s", ev.Host, ev.Msg))

	case parser.EventRunnerSkipped:
		if r.currentTask < len(r.tasks) {
			r.tasks[r.currentTask].skipped++
		}
		r.totalSkipped++

	case parser.EventStats:
		if ev.Stats != nil {
			// overwrite totals with authoritative final counts
			r.totalOK = sumMap(ev.Stats.Ok)
			r.totalChanged = sumMap(ev.Stats.Changed)
			r.totalFailed = sumMap(ev.Stats.Failed)
			r.totalSkipped = sumMap(ev.Stats.Skipped)
		}

	case parser.EventRaw:
		r.appendOutput(ev.Raw)
	}
	return r
}

func (r *Run) appendOutput(line string) {
	r.outputLines = append(r.outputLines, line)
}

func (r *Run) refreshOutputVP() {
	filtered := r.filteredOutput()
	r.outputVP.SetContent(strings.Join(filtered, "\n"))
	r.outputVP.GotoBottom()
}

func (r Run) filteredOutput() []string {
	if r.outputFilter == "" {
		return r.outputLines
	}
	var out []string
	for _, l := range r.outputLines {
		if strings.Contains(l, r.outputFilter) {
			out = append(out, l)
		}
	}
	return out
}

func (r *Run) resizeViewports() {
	outputH := r.height / 3
	r.outputVP.Width = r.width - 2
	r.outputVP.Height = outputH
}

func (r Run) View() string {
	if r.showHelp {
		return styles.InactiveBorder.Render(`RUN KEYBINDINGS

  esc    Cancel run (SIGINT)
  tab    Switch pane
  ↑ / k  Navigate tasks
  ↓ / j
  ?      Toggle help
  q      Quit
`)
	}

	pbName := r.opts.Playbook
	if idx := strings.LastIndex(pbName, "/"); idx >= 0 {
		pbName = pbName[idx+1:]
	}
	title := styles.TitleBar.Render(
		fmt.Sprintf("anvil  Running: %s  [Esc] cancel", pbName),
	)

	halfW := (r.width - 3) / 2
	taskPane := r.taskListPane(halfW, r.height/2-4)
	progressPane := r.progressPane(halfW, r.height/2-4)
	top := lipgloss.JoinHorizontal(lipgloss.Top, taskPane, " ", progressPane)

	outputPane := r.outputPane(r.width-2, r.height/3)
	statusBar := r.statusBarView()

	return lipgloss.JoinVertical(lipgloss.Left, title, top, outputPane, statusBar)
}

func (r Run) taskListPane(w, h int) string {
	s := styles.InactiveBorder
	if r.focus == 0 {
		s = styles.ActiveBorder
	}
	body := "TASKS\n"
	for i, t := range r.tasks {
		line := fmt.Sprintf("%s %-30s (%d/%d/%d)",
			t.icon(), t.name, t.ok, t.failed, t.inProgress)
		if i == r.taskIdx {
			line = styles.Selected.Render(line)
		}
		body += line + "\n"
	}
	return s.Width(w).Height(h).Render(body)
}

func (r Run) progressPane(w, h int) string {
	s := styles.InactiveBorder
	if r.focus == 1 {
		s = styles.ActiveBorder
	}
	body := "PROGRESS\n"
	// aggregate task bar per task
	for _, t := range r.tasks {
		total := t.ok + t.changed + t.failed + t.skipped
		bar := renderBar(t.ok, t.changed, t.failed, t.skipped, 20)
		body += fmt.Sprintf("  %-20s %s (%d)\n", t.name, bar, total)
	}
	return s.Width(w).Height(h).Render(body)
}

func (r Run) outputPane(w, h int) string {
	r.outputVP.Width = w - 2
	r.outputVP.Height = h - 2
	label := "OUTPUT"
	if r.outputFilter != "" {
		label += fmt.Sprintf(" (filter: %s)", r.outputFilter)
	}
	return styles.InactiveBorder.Width(w).Render(label + "\n" + r.outputVP.View())
}

func (r Run) statusBarView() string {
	elapsed := r.elapsed.Round(time.Second).String()
	bar := fmt.Sprintf("  %s %s  %s %s  %s %s  %s %s   PLAY: %s   %s",
		styles.OkCount.Render("✓"), fmt.Sprint(r.totalOK),
		styles.ChangedCount.Render("~"), fmt.Sprint(r.totalChanged),
		styles.FailCount.Render("✗"), fmt.Sprint(r.totalFailed),
		styles.SkipCount.Render("⊘"), fmt.Sprint(r.totalSkipped),
		r.currentPlay,
		styles.KeyHint.Render(elapsed),
	)
	return styles.StatusBar.Width(r.width).Render(bar)
}

// renderBar builds a coloured progress bar using lipgloss.
func renderBar(ok, changed, failed, skipped, width int) string {
	total := ok + changed + failed + skipped
	if total == 0 {
		return strings.Repeat("░", width)
	}
	seg := func(count int, color lipgloss.Color) string {
		n := count * width / total
		return lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", n))
	}
	return seg(ok, styles.Green) +
		seg(changed, styles.Yellow) +
		seg(failed, styles.Red) +
		seg(skipped, styles.Blue)
}

func sumMap(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}
