# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# anvil

TUI Ansible runner in Go using Bubble Tea.

## Build & test
- `go build ./...` to compile
- `go test ./...` to run all tests
- `go test ./internal/parser/...` to run a single package's tests
- `go vet ./...` before committing

## Code style
- Standard Go formatting (`gofmt`)
- Errors wrapped with `%w`
- No global state; pass config/state through model structs
- Bubble Tea pattern: every view is a `tea.Model` with `Init`, `Update`, `View`

## Key dependencies
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/charmbracelet/bubbles` — list, textinput, viewport, spinner
- `github.com/BurntSushi/toml` — config parsing

## Subprocess rules
- All ansible calls go through `internal/runner`
- Never call `exec.Command` directly outside of `runner/`
- Always context-cancel subprocesses on quit or Esc

## Ansible assumptions
- `ansible` and `ansible-playbook` are in PATH
- JSON stdout callback is set via env var (`ANSIBLE_STDOUT_CALLBACK=json`), not ansible.cfg

## Intended file structure

```
anvil/
├── cmd/anvil/main.go
├── internal/
│   ├── config/       # TOML config loading (~/.config/anvil/config.toml)
│   ├── inventory/    # ansible-inventory --list parsing
│   ├── runner/       # subprocess management + stdout streaming
│   ├── parser/       # Ansible JSON callback event classification
│   └── tui/
│       ├── app.go          # top-level Bubble Tea model; owns view transitions
│       ├── views/
│       │   ├── main.go     # playbook list + inventory tree + run options
│       │   ├── preview.go  # pre-run machine/variable review
│       │   ├── run.go      # task list + progress + output panel
│       │   └── picker.go   # project picker (shown only when >1 project)
│       └── styles/         # Lip Gloss style definitions
```

## View state machine

`ProjectPicker` → `MainView` → `PreviewView` → `RunView`

- `ProjectPicker` is skipped when only one project/default is configured.
- `PreviewView` runs `ansible-inventory --host <hostname>` per host to show variables; Esc returns to `MainView` with options intact.
- `RunView` replaces `PreviewView`; Esc sends SIGINT to the subprocess.

## Ansible integration

**Inventory** — loaded once on startup via:
```
ansible-inventory -i <inventory> --list --output /tmp/anvil-inv.json
```

**Run command** built as:
```
ansible-playbook -i <inventory> [--limit] [--tags] [--extra-vars] [--check] [--diff]
  [--vault-password-file] -e ANSIBLE_STDOUT_CALLBACK=json <playbook>
```

**Output parsing** (`internal/parser`) — one JSON object per stdout line; classify by event type:

| Event | Action |
|-------|--------|
| `v2_playbook_on_play_start` | New play in task list |
| `v2_playbook_on_task_start` | Add task, mark running |
| `v2_runner_on_ok` | Host ok |
| `v2_runner_on_changed` | Host changed |
| `v2_runner_on_failed` | Host failed |
| `v2_runner_on_skipped` | Host skipped |
| `v2_playbook_on_stats` | Final summary |

Stderr is captured separately and shown in a collapsible section in the output panel. JSON parse failures on individual lines are logged to a debug buffer; parsing continues.
