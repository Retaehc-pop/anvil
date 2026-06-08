package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Retaehc-pop/anvil/internal/config"
	"github.com/Retaehc-pop/anvil/internal/inventory"
	"github.com/Retaehc-pop/anvil/internal/parser"
	"github.com/Retaehc-pop/anvil/internal/runner"
	"github.com/Retaehc-pop/anvil/internal/tui/views"
)

type viewState int

const (
	statePicker  viewState = iota
	stateMain
	statePreview
	stateRun
)

// App is the root Bubble Tea model. It owns the current view and orchestrates
// transitions between picker → main → preview → run.
type App struct {
	cfg      *config.Config
	project  config.Project
	state    viewState
	invTree  *inventory.Tree

	picker  views.Picker
	main    views.Main
	preview views.Preview
	run     views.Run

	cancelRun context.CancelFunc

	width  int
	height int
}

func New(cfg *config.Config) App {
	projects := cfg.ActiveProjects()
	app := App{cfg: cfg}

	if len(projects) > 1 {
		app.state = statePicker
		app.picker = views.NewPicker(projects)
	} else {
		app.project = projects[0]
		app.state = stateMain
		app.main = views.NewMain(projects[0])
	}
	return app
}

func (a App) Init() tea.Cmd {
	switch a.state {
	case statePicker:
		return a.picker.Init()
	case stateMain:
		return tea.Batch(a.main.Init(), a.loadInventoryCmd())
	}
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return a.propagateSize(msg)

	// ── View transitions ──────────────────────────────────────────────────────

	case views.ProjectPickedMsg:
		a.project = msg.Project
		a.state = stateMain
		a.main = views.NewMain(msg.Project)
		m, sizeCmd := a.applySize(a.main)
		a.main = m.(views.Main)
		return a, tea.Batch(a.main.Init(), a.loadInventoryCmd(), sizeCmd)

	case views.InventoryLoadedMsg:
		a.invTree = msg.Tree
		var m tea.Model
		var cmd tea.Cmd
		m, cmd = a.main.Update(msg)
		a.main = m.(views.Main)
		return a, cmd

	case views.RunOptionsMsg:
		a.preview = views.NewPreview(a.project, msg, a.invTree)
		m, sizeCmd := a.applySize(a.preview)
		a.preview = m.(views.Preview)
		a.state = statePreview
		return a, tea.Batch(a.preview.Init(), a.fetchHostVarsCmd(a.preview), sizeCmd)

	case views.PreviewCancelMsg:
		a.state = stateMain
		return a, nil

	case views.PreviewRunMsg:
		opts := msg.Opts
		if msg.Dry {
			opts.Check = true
		}
		a.state = stateRun
		a.run = views.NewRun(opts)
		m, sizeCmd := a.applySize(a.run)
		a.run = m.(views.Run)
		ctx, cancel := context.WithCancel(context.Background())
		a.cancelRun = cancel
		return a, tea.Batch(a.run.Init(), a.startRunCmd(ctx, opts), sizeCmd)

	case views.RunDoneMsg:
		if a.cancelRun != nil {
			a.cancelRun()
		}
		// stay on run view to show final output; user presses q to quit
		var m tea.Model
		var cmd tea.Cmd
		m, cmd = a.run.Update(msg)
		a.run = m.(views.Run)
		return a, cmd

	case runLineMsg:
		var m tea.Model
		var cmd tea.Cmd
		m, cmd = a.run.Update(msg.line)
		a.run = m.(views.Run)
		return a, tea.Batch(cmd, drainCmd(msg.ch))

	case views.HostVarsLoadedMsg:
		var m tea.Model
		var cmd tea.Cmd
		m, cmd = a.preview.Update(msg)
		a.preview = m.(views.Preview)
		return a, cmd
	}

	// Delegate to active view
	return a.delegateToActive(msg)
}

func (a App) View() string {
	switch a.state {
	case statePicker:
		return a.picker.View()
	case stateMain:
		return a.main.View()
	case statePreview:
		return a.preview.View()
	case stateRun:
		return a.run.View()
	}
	return ""
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (a App) delegateToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	var m tea.Model
	var cmd tea.Cmd
	switch a.state {
	case statePicker:
		m, cmd = a.picker.Update(msg)
		a.picker = m.(views.Picker)
	case stateMain:
		m, cmd = a.main.Update(msg)
		a.main = m.(views.Main)
	case statePreview:
		m, cmd = a.preview.Update(msg)
		a.preview = m.(views.Preview)
	case stateRun:
		m, cmd = a.run.Update(msg)
		a.run = m.(views.Run)
	}
	return a, cmd
}

func (a App) propagateSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	a.width, a.height = msg.Width, msg.Height
	var cmds []tea.Cmd
	delegate := func(m tea.Model) tea.Model {
		updated, cmd := m.Update(msg)
		cmds = append(cmds, cmd)
		return updated
	}
	a.picker = delegate(a.picker).(views.Picker)
	a.main = delegate(a.main).(views.Main)
	if a.state == statePreview {
		a.preview = delegate(a.preview).(views.Preview)
	}
	if a.state == stateRun {
		a.run = delegate(a.run).(views.Run)
	}
	return a, tea.Batch(cmds...)
}

// applySize sends the current terminal dimensions to a newly created view so it
// renders correctly without waiting for the next WindowSizeMsg.
func (a App) applySize(m tea.Model) (tea.Model, tea.Cmd) {
	if a.width == 0 {
		return m, nil
	}
	return m.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
}

func (a App) loadInventoryCmd() tea.Cmd {
	inv := a.project.Inventory
	return func() tea.Msg {
		tree, err := inventory.Load(context.Background(), inv)
		return views.InventoryLoadedMsg{Tree: tree, Err: err}
	}
}

func (a App) fetchHostVarsCmd(pv views.Preview) tea.Cmd {
	var cmds []tea.Cmd
	for _, host := range pv.Hosts() {
		h := host
		cmds = append(cmds, func() tea.Msg {
			vars, err := inventory.FetchHostVars(context.Background(), a.project.Inventory, h)
			return views.HostVarsLoadedMsg{Host: h, Vars: vars, Err: err}
		})
	}
	return tea.Batch(cmds...)
}

func (a App) startRunCmd(ctx context.Context, opts views.RunOptionsMsg) tea.Cmd {
	proj := a.project
	runOpts := runner.Options{
		Playbook:  opts.Playbook,
		Limit:     opts.Limit,
		Tags:      opts.Tags,
		ExtraVars: opts.ExtraVars,
		Check:     opts.Check,
		Diff:      opts.Diff,
		Format:    parser.FormatJSON,
	}
	ch := make(chan runner.LineMsg, 256)

	// Start subprocess in background; channel is closed when it exits.
	go func() {
		runner.Run(ctx, proj, runOpts, ch) //nolint:errcheck
	}()

	// Return first drain cmd; each subsequent one is scheduled from Update.
	return drainCmd(ch)
}

// drainCmd reads exactly one event from ch and returns it as a tea.Msg.
// If the channel is closed it returns RunDoneMsg. The App.Update schedules
// another drainCmd after each message, creating a pull-based stream loop.
func drainCmd(ch <-chan runner.LineMsg) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return views.RunDoneMsg{}
		}
		return runLineMsg{line: line, ch: ch}
	}
}

// runLineMsg wraps a runner.LineMsg together with the channel so Update can
// schedule the next drain without closing over a stale reference.
type runLineMsg struct {
	line runner.LineMsg
	ch   <-chan runner.LineMsg
}
