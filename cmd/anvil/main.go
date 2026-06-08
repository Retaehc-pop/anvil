package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Retaehc-pop/anvil/internal/config"
	"github.com/Retaehc-pop/anvil/internal/tui"
)

const version = "0.1.0"

const helpText = `anvil — TUI Ansible runner

Usage:
  anvil [flags]

Flags:
  -h, --help     Show this help message
  -v, --version  Print version

Config:
  ~/.config/anvil/config.toml  (or $XDG_CONFIG_HOME/anvil/config.toml)

  [defaults]
  inventory          = "~/infra/inventory"
  playbook_dir       = "~/infra/playbooks"
  vault_password_file = "~/.vault_pass"

  [[project]]
  name               = "my-project"
  inventory          = "/path/to/inventory"
  playbook_dir       = "/path/to/playbooks"

Requirements:
  ansible and ansible-playbook must be in PATH.

Keybindings (main view):
  Tab        Switch focus between playbook list and inventory tree
  Enter      Open preview
  l          Focus limit field
  t          Focus tags field
  e          Focus extra-vars field
  c          Toggle check mode / clear options (outside input)
  d          Toggle diff mode
  ?          Toggle help overlay
  q          Quit

Keybindings (run view):
  Esc        Cancel run
  Tab        Switch focus panel
  f          Filter output by hostname
  s          Save output to file
`

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-h", "--help":
			fmt.Print(helpText)
			os.Exit(0)
		case "-v", "--version":
			fmt.Printf("anvil %s\n", version)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "anvil: unknown flag: %s\n\nRun 'anvil --help' for usage.\n", arg)
			os.Exit(1)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "anvil: config error: %v\n", err)
		os.Exit(1)
	}

	app := tui.New(cfg)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "anvil: %v\n", err)
		os.Exit(1)
	}
}
