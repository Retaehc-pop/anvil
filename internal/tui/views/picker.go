package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Retaehc-pop/anvil/internal/config"
	"github.com/Retaehc-pop/anvil/internal/tui/styles"
)

// ProjectPickedMsg is sent when the user selects a project.
type ProjectPickedMsg struct {
	Project config.Project
}

// Picker is the project selection screen shown when >1 project is configured.
type Picker struct {
	projects []config.Project
	cursor   int
	width    int
	height   int
}

func NewPicker(projects []config.Project) Picker {
	return Picker{projects: projects}
}

func (p Picker) Init() tea.Cmd { return nil }

func (p Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.projects)-1 {
				p.cursor++
			}
		case "enter", " ":
			return p, func() tea.Msg {
				return ProjectPickedMsg{Project: p.projects[p.cursor]}
			}
		case "q", "ctrl+c":
			return p, tea.Quit
		}
	}
	return p, nil
}

func (p Picker) View() string {
	title := styles.TitleBar.Render("anvil  — select project")

	items := ""
	for i, proj := range p.projects {
		line := fmt.Sprintf("  %s", proj.Name)
		if i == p.cursor {
			line = styles.Selected.Render(fmt.Sprintf("> %s", proj.Name))
		}
		items += line + "\n"
	}

	hint := styles.KeyHint.Render("↑/↓ navigate  enter select  q quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		items,
		"",
		hint,
	)
}
