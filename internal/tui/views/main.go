package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Retaehc-pop/anvil/internal/config"
	"github.com/Retaehc-pop/anvil/internal/inventory"
	"github.com/Retaehc-pop/anvil/internal/tui/styles"
)

type focusPane int

const (
	panePlaybooks focusPane = iota
	paneInventory
	paneOptions
)

// RunOptionsMsg is sent when the user presses Enter to go to Preview.
type RunOptionsMsg struct {
	Playbook  string
	Limit     string
	Tags      string
	ExtraVars string
	Check     bool
	Diff      bool
}

// InventoryLoadedMsg carries the parsed inventory tree.
type InventoryLoadedMsg struct {
	Tree *inventory.Tree
	Err  error
}

// inventoryItem is a flat row in the tree view (group or host + indent level).
type inventoryItem struct {
	node  *inventory.Node
	depth int
}

// Main is the primary screen: playbook list + inventory tree + run options.
type Main struct {
	project   config.Project
	width     int
	height    int
	focus     focusPane
	showHelp  bool

	// Playbooks
	playbooks    []string
	playbookIdx  int

	// Inventory tree (flat, visible rows)
	invTree  *inventory.Tree
	invRows  []inventoryItem
	invIdx   int
	expanded map[string]bool

	// Run options inputs
	limitInput  textinput.Model
	tagsInput   textinput.Model
	extraInput  textinput.Model
	checkMode   bool
	diffMode    bool
	optionFocus int // 0=limit 1=tags 2=extra 3=check 4=diff
}

func NewMain(proj config.Project) Main {
	li := textinput.New()
	li.Placeholder = "all"
	li.Width = 20

	ti := textinput.New()
	ti.Placeholder = "tag1,tag2"
	ti.Width = 20

	ei := textinput.New()
	ei.Placeholder = "key=val"
	ei.Width = 20

	m := Main{
		project:    proj,
		focus:      panePlaybooks,
		limitInput: li,
		tagsInput:  ti,
		extraInput: ei,
		expanded:   map[string]bool{"all": true},
	}
	m.playbooks = scanPlaybooks(proj.PlaybookDir)
	return m
}

func (m Main) Init() tea.Cmd {
	return nil
}

func (m Main) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case InventoryLoadedMsg:
		if msg.Err == nil && msg.Tree != nil {
			m.invTree = msg.Tree
			m.invRows = flattenTree(msg.Tree.Root, 0, m.expanded)
		}

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			prev := m.focus
			m.focus = (m.focus + 1) % 3
			m.syncPanelChange(prev)
			return m, nil
		case "shift+tab":
			prev := m.focus
			m.focus = (m.focus + 2) % 3
			m.syncPanelChange(prev)
			return m, nil
		}

		switch m.focus {
		case panePlaybooks:
			m = m.updatePlaybooks(msg)
		case paneInventory:
			m = m.updateInventory(msg)
		case paneOptions:
			var cmd tea.Cmd
			m, cmd = m.updateOptions(msg)
			return m, cmd
		}
	}
	return m, nil
}

// syncPanelChange handles blur/focus of text inputs when switching panels.
func (m *Main) syncPanelChange(prev focusPane) {
	if prev == paneOptions {
		m.limitInput.Blur()
		m.tagsInput.Blur()
		m.extraInput.Blur()
	}
	if m.focus == paneOptions {
		m.syncOptionFocus()
	}
}

func (m Main) updatePlaybooks(msg tea.KeyMsg) Main {
	switch msg.String() {
	case "up", "k":
		if m.playbookIdx > 0 {
			m.playbookIdx--
		}
	case "down", "j":
		if m.playbookIdx < len(m.playbooks)-1 {
			m.playbookIdx++
		}
	}
	return m
}

func (m Main) updateInventory(msg tea.KeyMsg) Main {
	switch msg.String() {
	case "up", "k":
		if m.invIdx > 0 {
			m.invIdx--
		}
	case "down", "j":
		if m.invIdx < len(m.invRows)-1 {
			m.invIdx++
		}
	case " ":
		if m.invIdx < len(m.invRows) {
			row := m.invRows[m.invIdx]
			if row.node.IsGroup {
				m.expanded[row.node.Name] = !m.expanded[row.node.Name]
				m.invRows = flattenTree(m.invTree.Root, 0, m.expanded)
			}
		}
	case "enter", "l":
		if m.invIdx < len(m.invRows) {
			m.limitInput.SetValue(m.invRows[m.invIdx].node.Name)
		}
	}
	return m
}

func (m Main) updateOptions(msg tea.KeyMsg) (Main, tea.Cmd) {
	switch msg.String() {
	case "down":
		// cycle Limit(0)→Tags(1)→Extra(2)→Check(3)→Diff(4)→Limit; stays in panel
		m.optionFocus = (m.optionFocus + 1) % 5
		m.syncOptionFocus()
		return m, nil
	case "up":
		m.optionFocus = (m.optionFocus + 4) % 5
		m.syncOptionFocus()
		return m, nil
	case " ":
		switch m.optionFocus {
		case 3:
			m.checkMode = !m.checkMode
		case 4:
			m.diffMode = !m.diffMode
		}
		return m, nil
	case "c":
		m.limitInput.SetValue("")
		m.tagsInput.SetValue("")
		m.extraInput.SetValue("")
		m.checkMode = false
		m.diffMode = false
		return m, nil
	case "enter":
		if len(m.playbooks) == 0 {
			return m, nil
		}
		pb := filepath.Join(m.project.PlaybookDir, m.playbooks[m.playbookIdx])
		return m, func() tea.Msg {
			return RunOptionsMsg{
				Playbook:  pb,
				Limit:     m.limitInput.Value(),
				Tags:      m.tagsInput.Value(),
				ExtraVars: m.extraInput.Value(),
				Check:     m.checkMode,
				Diff:      m.diffMode,
			}
		}
	}

	var cmd tea.Cmd
	switch m.optionFocus {
	case 0:
		m.limitInput, cmd = m.limitInput.Update(msg)
	case 1:
		m.tagsInput, cmd = m.tagsInput.Update(msg)
	case 2:
		m.extraInput, cmd = m.extraInput.Update(msg)
	}
	return m, cmd
}

func (m *Main) syncOptionFocus() {
	m.limitInput.Blur()
	m.tagsInput.Blur()
	m.extraInput.Blur()
	switch m.optionFocus {
	case 0:
		m.limitInput.Focus()
	case 1:
		m.tagsInput.Focus()
	case 2:
		m.extraInput.Focus()
	// 3=Check, 4=Diff: no text input active; toggled with space
	}
}

func (m Main) View() string {
	if m.showHelp {
		return m.helpView()
	}

	title := styles.TitleBar.Render(fmt.Sprintf("anvil  [%s]", m.project.Name)) +
		styles.KeyHint.Render("  [?] Help  [Q] Quit")

	// title(1) + actions(1) + opts rendered(3+2=5) + pane borders(2) = 9
	halfW := (m.width - 5) / 2
	paneH := m.height - 9

	pbPane := m.playbookPane(halfW, paneH)
	invPane := m.inventoryPane(halfW, paneH)
	top := lipgloss.JoinHorizontal(lipgloss.Top, pbPane, " ", invPane)

	opts := m.optionsPane(m.width - 2)

	var actions string
	switch m.focus {
	case panePlaybooks:
		actions = styles.KeyHint.Render("  [↑↓] Navigate   [Tab] Next panel   [Shift+Tab] Prev panel")
	case paneInventory:
		actions = styles.KeyHint.Render("  [↑↓] Navigate   [Space] Expand   [Enter] Set limit   [Tab] Next panel")
	case paneOptions:
		actions = styles.KeyHint.Render("  [↑↓] Navigate fields   [Space] Toggle   [Enter] Preview   [C] Clear   [Tab] Next panel")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, top, opts, actions)
}

func (m Main) playbookPane(w, h int) string {
	s := styles.InactiveBorder
	if m.focus == panePlaybooks {
		s = styles.ActiveBorder
	}
	title := "PLAYBOOKS\n"
	body := ""
	for i, pb := range m.playbooks {
		line := "  " + pb
		if i == m.playbookIdx {
			line = styles.Selected.Render("> " + pb)
		}
		body += line + "\n"
	}
	if len(m.playbooks) == 0 {
		body = styles.KeyHint.Render("  (no playbooks found)")
	}
	return s.Width(w).Height(h).Render(title + body)
}

func (m Main) inventoryPane(w, h int) string {
	s := styles.InactiveBorder
	if m.focus == paneInventory {
		s = styles.ActiveBorder
	}
	title := "INVENTORY\n"
	body := ""
	if m.invTree == nil {
		body = styles.KeyHint.Render("  (loading…)")
	} else {
		for i, row := range m.invRows {
			indent := strings.Repeat("  ", row.depth)
			icon := "  "
			if row.node.IsGroup {
				if m.expanded[row.node.Name] {
					icon = "▼ "
				} else {
					icon = "▶ "
				}
			}
			line := fmt.Sprintf("%s%s%s", indent, icon, row.node.Name)
			if i == m.invIdx && m.focus == paneInventory {
				line = styles.Selected.Render(line)
			}
			body += line + "\n"
		}
	}
	return s.Width(w).Height(h).Render(title + body)
}

func (m Main) optionsPane(w int) string {
	checkBox := "[ ]"
	if m.checkMode {
		checkBox = "[x]"
	}
	diffBox := "[ ]"
	if m.diffMode {
		diffBox = "[x]"
	}
	checkLabel := "Check: " + checkBox
	diffLabel := "Diff: " + diffBox
	if m.focus == paneOptions && m.optionFocus == 3 {
		checkLabel = styles.Selected.Render(checkLabel)
	}
	if m.focus == paneOptions && m.optionFocus == 4 {
		diffLabel = styles.Selected.Render(diffLabel)
	}
	line1 := fmt.Sprintf("  Limit: [%s]  Tags: [%s]", m.limitInput.View(), m.tagsInput.View())
	line2 := fmt.Sprintf("  Extra: [%s]  %s  %s", m.extraInput.View(), checkLabel, diffLabel)
	s := styles.InactiveBorder
	if m.focus == paneOptions {
		s = styles.ActiveBorder
	}
	return s.Width(w).Render("RUN OPTIONS\n" + line1 + "\n" + line2)
}

func (m Main) helpView() string {
	help := `KEYBINDINGS

  All panels:
  Tab            Next panel
  Shift+Tab      Previous panel
  ?              Toggle help
  Q              Quit

  Playbooks / Inventory:
  ↑ / K          Move up
  ↓ / J          Move down
  Space          Expand/collapse group  (inventory)
  Enter          Set host/group as limit  (inventory)

  Run Options:
  ↑ / ↓          Navigate fields  (Limit → Tags → Extra → Check → Diff)
  Space          Toggle Check / Diff when focused
  C              Clear all options
  Enter          Go to Preview
`
	return styles.InactiveBorder.Render(help)
}

// flattenTree converts the tree into a flat slice respecting expand state.
func flattenTree(node *inventory.Node, depth int, expanded map[string]bool) []inventoryItem {
	rows := []inventoryItem{{node: node, depth: depth}}
	if node.IsGroup && expanded[node.Name] {
		for _, child := range node.Children {
			rows = append(rows, flattenTree(child, depth+1, expanded)...)
		}
	}
	return rows
}

func scanPlaybooks(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			out = append(out, name)
		}
	}
	return out
}
