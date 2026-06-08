package views

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Retaehc-pop/anvil/internal/config"
	"github.com/Retaehc-pop/anvil/internal/inventory"
	"github.com/Retaehc-pop/anvil/internal/tui/styles"
)

// PreviewRunMsg is sent when the user presses r (run) or d (dry run).
type PreviewRunMsg struct {
	Opts RunOptionsMsg
	Dry  bool
}

// PreviewCancelMsg is sent when the user presses Esc to return to Main.
type PreviewCancelMsg struct{}

// HostVarsLoadedMsg is sent when host vars are fetched from ansible-inventory.
type HostVarsLoadedMsg struct {
	Host string
	Vars map[string]any
	Err  error
}

// Preview shows the list of matched hosts and their variables before a run.
type Preview struct {
	project  config.Project
	opts     RunOptionsMsg
	width    int
	height   int
	showHelp bool

	// hosts derived from inventory
	hosts    []string
	hostVars map[string]map[string]any
	hostIdx  int

	focusLeft   bool // true = machine list, false = variable panel
	showConfirm bool
}

func NewPreview(proj config.Project, opts RunOptionsMsg, tree *inventory.Tree) Preview {
	hosts := resolveHosts(opts.Limit, tree)
	return Preview{
		project:   proj,
		opts:      opts,
		hosts:     hosts,
		hostVars:  map[string]map[string]any{},
		focusLeft: true,
	}
}

// Hosts returns the list of matched hosts for this preview session.
func (p Preview) Hosts() []string { return p.hosts }

func (p Preview) Init() tea.Cmd {
	return nil
}

func (p Preview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height

	case HostVarsLoadedMsg:
		if msg.Err == nil {
			p.hostVars[msg.Host] = msg.Vars
		}

	case tea.KeyMsg:
		if p.showConfirm {
			switch msg.String() {
			case "enter", "r":
				return p, func() tea.Msg { return PreviewRunMsg{Opts: p.opts, Dry: false} }
			case "d":
				return p, func() tea.Msg { return PreviewRunMsg{Opts: p.opts, Dry: true} }
			case "esc", "q", "ctrl+c":
				p.showConfirm = false
			}
			return p, nil
		}

		switch msg.String() {
		case "?":
			p.showHelp = !p.showHelp
		case "q", "ctrl+c":
			return p, tea.Quit
		case "esc":
			return p, func() tea.Msg { return PreviewCancelMsg{} }
		case "r":
			return p, func() tea.Msg { return PreviewRunMsg{Opts: p.opts, Dry: false} }
		case "d":
			return p, func() tea.Msg { return PreviewRunMsg{Opts: p.opts, Dry: true} }
		case "tab", "enter":
			if p.focusLeft {
				p.focusLeft = false // machine → variables
			} else {
				p.showConfirm = true // variables → confirm dialog
			}
		case "shift+tab":
			p.focusLeft = true // always go back to machine
		case "up", "k":
			if p.focusLeft && p.hostIdx > 0 {
				p.hostIdx--
			}
		case "down", "j":
			if p.focusLeft && p.hostIdx < len(p.hosts)-1 {
				p.hostIdx++
			}
		}
	}
	return p, nil
}

func (p Preview) View() string {
	if p.showHelp {
		return styles.InactiveBorder.Render(`PREVIEW KEYBINDINGS

  Tab / Shift+Tab   Switch panel
  ↑ / K             Navigate hosts
  R                 Run
  D                 Dry run (forces --check)
  Esc               Back to main view
  ?                 Toggle help
  Q                 Quit
`)
	}

	pbName := p.opts.Playbook
	if idx := strings.LastIndex(pbName, "/"); idx >= 0 {
		pbName = pbName[idx+1:]
	}
	limitLabel := p.opts.Limit
	if limitLabel == "" {
		limitLabel = "all"
	}

	header := styles.TitleBar.Render(fmt.Sprintf("anvil  [%s]", p.project.Name)) +
		styles.KeyHint.Render("  [?] Help  [Q] Quit")

	playbookLine := fmt.Sprintf("  PLAYBOOK: %s  →  %s", pbName, limitLabel)

	optLine := fmt.Sprintf("  Limit: [%s]  Tags: [%s]  Extra: [%s]  Check: [%s]  Diff: [%s]",
		limitLabel, p.opts.Tags, p.opts.ExtraVars,
		boolStr(p.opts.Check), boolStr(p.opts.Diff),
	)

	// header(1) + playbookLine(1) + optLine(1) + actions(1) + pane borders(2) = 6
	// 30/70 split: total content width = p.width - 5 (4 borders + 1 separator)
	totalW := p.width - 5
	machineW := totalW * 3 / 10
	varW := totalW - machineW
	paneH := p.height - 6

	machinePane := p.machineList(machineW, paneH)
	varPane := p.varPanel(varW, paneH)
	mid := lipgloss.JoinHorizontal(lipgloss.Top, machinePane, " ", varPane)

	actions := styles.KeyHint.Render("  [R] Run   [D] Dry Run   [Esc] Back   [Tab/Enter] Next   [Shift+Tab] Prev")

	view := lipgloss.JoinVertical(lipgloss.Left, header, playbookLine, optLine, mid, actions)

	if p.showConfirm {
		view = lipgloss.Place(p.width, p.height,
			lipgloss.Center, lipgloss.Center,
			p.confirmDialog(pbName, limitLabel),
		)
	}
	return view
}

func (p Preview) confirmDialog(pbName, limitLabel string) string {
	check := "no"
	if p.opts.Check {
		check = "yes"
	}
	diff := "no"
	if p.opts.Diff {
		diff = "yes"
	}
	body := fmt.Sprintf(
		"  Playbook : %s\n  Limit    : %s\n  Check    : %s\n  Diff     : %s",
		pbName, limitLabel, check, diff,
	)
	if p.opts.Tags != "" {
		body += fmt.Sprintf("\n  Tags     : %s", p.opts.Tags)
	}
	if p.opts.ExtraVars != "" {
		body += fmt.Sprintf("\n  Extra    : %s", p.opts.ExtraVars)
	}
	content := styles.TitleBar.Render("  Confirm Run  ") + "\n\n" +
		body + "\n\n" +
		styles.KeyHint.Render("  [Enter] Run   [D] Dry run   [Esc] Cancel")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Blue).
		Padding(1, 2).
		Render(content)
}

func (p Preview) machineList(w, h int) string {
	s := styles.InactiveBorder
	if p.focusLeft {
		s = styles.ActiveBorder
	}
	title := fmt.Sprintf("Machine (%d)\n", len(p.hosts))
	body := ""
	for i, host := range p.hosts {
		line := "  " + host
		if i == p.hostIdx {
			line = styles.Selected.Render("> " + host)
		}
		body += line + "\n"
	}
	if len(p.hosts) == 0 {
		body = styles.KeyHint.Render("  (no hosts matched)")
	}
	return s.Width(w).Height(h).Render(title + body)
}

func (p Preview) varPanel(w, h int) string {
	s := styles.InactiveBorder
	if !p.focusLeft {
		s = styles.ActiveBorder
	}
	title := "Variables\n"
	body := ""
	if len(p.hosts) == 0 {
		body = styles.KeyHint.Render("  (no host selected)")
	} else {
		host := p.hosts[p.hostIdx]
		vars, ok := p.hostVars[host]
		if !ok {
			body = styles.KeyHint.Render("  (loading vars…)")
		} else {
			keys := make([]string, 0, len(vars))
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := fmt.Sprintf("%v", vars[k])
				if strings.Contains(strings.ToLower(k), "pass") ||
					strings.Contains(strings.ToLower(k), "vault") ||
					strings.Contains(strings.ToLower(k), "secret") {
					v = "***"
				}
				body += fmt.Sprintf("  %-24s %s\n", k+":", v)
			}
		}
	}
	return s.Width(w).Height(h).Render(title + body)
}

// resolveHosts returns all hosts in the tree that match the limit expression.
// Currently supports a single group name, single host name, or empty (= all).
func resolveHosts(limit string, tree *inventory.Tree) []string {
	if tree == nil {
		return nil
	}
	if limit == "" || limit == "all" {
		hosts := make([]string, 0, len(tree.Hosts))
		for h := range tree.Hosts {
			hosts = append(hosts, h)
		}
		sort.Strings(hosts)
		return hosts
	}
	// exact host match
	if _, ok := tree.Hosts[limit]; ok {
		return []string{limit}
	}
	// group match — walk tree
	node := findGroup(tree.Root, limit)
	if node == nil {
		return []string{limit} // unknown, show as-is
	}
	return collectHosts(node)
}

func findGroup(node *inventory.Node, name string) *inventory.Node {
	if node.Name == name {
		return node
	}
	for _, c := range node.Children {
		if found := findGroup(c, name); found != nil {
			return found
		}
	}
	return nil
}

func collectHosts(node *inventory.Node) []string {
	if !node.IsGroup {
		return []string{node.Name}
	}
	seen := map[string]bool{}
	var walk func(*inventory.Node)
	walk = func(n *inventory.Node) {
		if !n.IsGroup {
			seen[n.Name] = true
			return
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(node)
	hosts := make([]string, 0, len(seen))
	for h := range seen {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

func boolStr(b bool) string {
	if b {
		return "x"
	}
	return " "
}
