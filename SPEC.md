# anvil — TUI Ansible Runner

## Overview

`anvil` is a terminal UI for Ansible, written in Go using Bubble Tea. It wraps the
`ansible` and `ansible-playbook` CLI tools, providing an interactive interface for
browsing inventories, selecting and running playbooks, filtering by tags and hosts,
and watching task execution in real time, without leaving the terminal.

Target users: ops engineers who already know Ansible but want faster iteration and
better visibility than raw CLI output.

---

## Goals

- Run playbooks interactively without typing long `ansible-playbook` invocations
- Visualize variable that will apply to each machine when running playbooks
- Browse and filter inventory (hosts, groups, variables)
- Visualize stage in which each machine is going through
- Reduce time spent constructing `--limit`, `--tags`, `--extra-vars` flags manually
- Stay in the terminal; no browser, no GUI

---

## Non-Goals

- Not a replacement for Ansible Tower / AWX
- No remote inventory editing
- No playbook authoring / YAML editing
- No multi-project workspace management

---

## Architecture

### Stack

| Component   | Choice                            |
|-------------|-----------------------------------|
| Language    | Go                                |
| TUI         | Bubble Tea + Lip Gloss            |
| Config      | TOML (`~/.config/anvil/config.toml`) |
| Ansible I/O | `os/exec` wrapping `ansible-playbook`, `ansible-inventory` |
| Output parse| Ansible JSON callback plugin (stdout) |

### Process model

`anvil` spawns `ansible-playbook` as a subprocess with
`ANSIBLE_STDOUT_CALLBACK=json` (or `yaml`). It reads stdout line-by-line,
parses Ansible's structured JSON output, and maps events onto its TUI panels
in real time. No Ansible Python API is used — pure subprocess.

---

## Configuration

Location: `~/.config/anvil/config.toml`

```toml
[defaults]
inventory = "~/infra/inventory"
playbook_dir = "~/infra/playbooks"
vault_password_file = "~/.vault_pass"

[[project]]
name = "hpc-cluster"
inventory = "/opt/cluster/inventory"
playbook_dir = "/opt/cluster/playbooks"
vault_password_file = "/opt/cluster/.vault_pass"

[[project]]
name = "monitoring"
inventory = "/opt/monitoring/inventory"
playbook_dir = "/opt/monitoring/playbooks"
```

On startup `anvil` loads the default config. If multiple projects are defined,
a project picker is shown first.

---

## UI Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ anvil  [hpc-cluster]                            [?] help  [q] quit │
├──────────────────┬──────────────────────────────────────────────┤
│  PLAYBOOKS       │  INVENTORY                                    │
│                  │                                               │
│  > site.yml      │  ▶ all (42 hosts)                            │
│    deploy.yml    │    ▼ cluster                                  │
│    update.yml    │      ▼ compute                                │
│    baseline.yml  │          node[01-32]                          │
│                  │        login                                  │
│                  │          login01                              │
│                  │          login02                              │
│                  │      storage                                  │
│                  │          nas01                               │
├──────────────────┴──────────────────────────────────────────────┤
│  RUN OPTIONS                                                    │
│  Limit:  [all              ]  Tags:    [              ]         │
│  Extra:  [                 ]  Check:   [ ] Diff: [ ]            │
├─────────────────────────────────────────────────────────────────┤
│  [ Preview (Enter) ]                [ Clear (c) ]               │
└─────────────────────────────────────────────────────────────────┘
```
When Preview
```
┌─────────────────────────────────────────────────────────────────┐
│ anvil  [hpc-cluster]                         [?] help  [q] quit │
├─────────────────────┬───────────────────────────────────────────┤
│  PLAYBOOKS: XXX.yml │  INVENTORY: cluster                       │
├────────────────────────────────────────────────────────────────┤
│  Limit:  [compute]    Tags: [...]  Extra:[...]                  │
│  Check:  [ ]          Diff: [x]                                 │
├──────────────────┬──────────────────────────────────────────────┤
│  Machine(1/32)   │  Variables                                    │
│  > node01        │  - ICINGA_SATELLITE:    asd                  │
│    node02        │  - VAULT_ICINGA_PW :    ***                  │
│    node03        │  - GROUP_HOST      :    zone1                │
│                  │                                              │
├─────────────────────────────────────────────────────────────────┤
│  [ Run (r) ]   [ Dry Run (d) ]    [ Cancle (esc) ]              │
└─────────────────────────────────────────────────────────────────┘
```


When a run is active, the layout switches to:

```
┌─────────────────────────────────────────────────────────────────┐
│ anvil  [hpc-cluster]  Running: site.yml          [Esc] cancel    │
├────────────────────┬────────────────────────────────────────────┤
│  TASKS                     │  PROGRESS                                   │
│                            │                                              │
│  ✓ Gathering Facts (32/0/0)│  node01: ======   (6/6)                          │
│  ~ Install pkgs    (30/2/0)│  node02: ======   (4/6)                         │
│  · Apply config    (30/1/1)│  node03: ======   (5/6)                          │
│  · Restart svc     (30/0/2)│  node04: ======   (6/6)                         │
│  · Verify          (30/0/2)│  node05: ======   (6/6)                    │
│                            │                                              │
│  [32 hosts]                │  node06  ↑ scroll  ↓                        │
├────────────────────┬────────────────────────────────────────────┤
│ OUTPUT                                                          │
│ TASK [Install packages] ────────────── 
  ok: [node01]
  ok: [node02]                                                    │
│                                                   ↑ scroll  ↓   │
├────────────────────┴────────────────────────────────────────────┤
│  ✓ 30  ✗ 2  ~ 0   PLAY: Configure cluster  TASK: Apply config   │
└─────────────────────────────────────────────────────────────────┘
```


---

## Views

### 1. Project Picker (if multiple projects configured)

- Simple list; arrow keys + Enter to select
- Skipped if only one project / default configured

### 2. Main View

Two panes, Tab to switch focus:

**Left — Playbook List**
- Scans `playbook_dir` for `*.yml` / `*.yaml` files at top level
- Arrow keys to navigate, Enter to select
- Selected playbook highlighted

**Right — Inventory Tree**
- Populated by running `ansible-inventory --list` on startup
- Tree view: All → groups → hosts
- Space to expand/collapse group
- Enter on a host or group to set it as the `--limit` value in Run Options

**Bottom — Run Options**

| Field | Key | Description |
|-------|-----|-------------|
| Limit | `l` | Focus limit field; free text or populated from inventory click |
| Tags  | `t` | Comma-separated tags |
| Extra Vars | `e` | `key=value` pairs, space-separated |
| Check mode | `c` | Toggle `--check` |
| Diff mode | `d` | Toggle `--diff` |

**Actions**

| Key | Action |
|-----|--------|
| Enter | Go to preview|
| `c` | Clear run options |
| `?` | Toggle help overlay |
| `q` | Quit |

### 3. Preview View

activate before running. Replace main view
Tab switching between machine and variable user arrow key to navigate
**Actions**

| Key | Action |
|-----|--------|
| `r` | Run |
| `d` | Dry run, force check mode |
| `esc` | go back to Main View with same configuration to tuning |
| `?` | Toggle help overlay |
| `q` | Quit |



### 4. Run View

Activated when a playbook starts. Replaces preview view.

**Left — Task List**
- One row per task name from the play
- Status icon: `·` pending, `✓` ok, `~` partial failed, `✗` all failed, `⊘` skipped
- Shows host count
- Show count status (OK/ERR/in process)

**Right — Progress List**
each = represent each task. = is green if OK, red if err, blue if skip, yellow if changed, gray if inprogress.

**Bottom - OUTPUT**
- Streams JSON-parsed task output
- Filterable by host (press `f`, type hostname)
- Scrollable; `PgUp`/`PgDn`, `g`/`G` for top/bottom
- Click/arrow to a task in left pane to jump output to that task's section

**Status Bar**
- Running totals: ok / changed / failed / unreachable / skipped
- Current play name + current task name
- Elapsed time

**Keys during run**

| Key | Action |
|-----|--------|
| Esc | Cancel run (sends SIGINT to subprocess) |
| `f` | Filter output by hostname |
| `Tab` | Switch focus panel |
| `s` | Save full output to file |

### 4. Host Detail Overlay

Press Enter on a host in the inventory tree to open a modal showing:
- All host variables (from `ansible-inventory --host <hostname>`)
- Group memberships
- `[c]` to copy hostname to clipboard

### 5. Help Overlay

`?` from any view. Shows all keybindings for current context.

---

## Ansible Integration

### Inventory parsing

```
ansible-inventory -i <inventory> --list --output /tmp/anvil-inv.json
```

Parsed once on startup; refreshable with `r`.

### Run command construction

```
ansible-playbook \
  -i <inventory> \
  [--limit <limit>] \
  [--tags <tags>] \
  [--extra-vars "<extra>"] \
  [--check] [--diff] \
  [--vault-password-file <file>] \
  -e ANSIBLE_STDOUT_CALLBACK=json \
  <playbook>
```

### Output parsing

Ansible JSON callback emits one JSON object per line to stdout. `anvil`
reads stdout line-by-line and classifies each event:

| Event type | Action |
|------------|--------|
| `v2_playbook_on_play_start` | New play heading in task list |
| `v2_playbook_on_task_start` | Add task to task list, mark running |
| `v2_runner_on_ok` | Mark host ok for current task |
| `v2_runner_on_changed` | Mark host changed |
| `v2_runner_on_failed` | Mark host failed, append error detail |
| `v2_runner_on_skipped` | Mark host skipped |
| `v2_playbook_on_stats` | Final summary; update status bar |

Stderr from the subprocess is captured separately and shown in a
collapsible "Ansible stderr" section at the bottom of the output panel
(useful for connection errors, vault issues, etc.).

---

## Error States

| Condition | Handling |
|-----------|----------|
| `ansible-playbook` not in PATH | Show error on startup with install hint |
| Inventory file not found | Show error in inventory pane, allow manual path edit |
| Vault password file missing | Warn before run; offer to run without vault |
| Subprocess exits non-zero | Show exit code + last stderr lines in status bar |
| JSON parse failure on a line | Log raw line to debug buffer; continue parsing |

---

## File Structure

```
anvil/
├── cmd/
│   └── anvil/
│       └── main.go
├── internal/
│   ├── config/         # TOML config loading
│   ├── inventory/      # ansible-inventory parsing
│   ├── runner/         # subprocess management, output streaming
│   ├── parser/         # Ansible JSON callback event parsing
│   └── tui/
│       ├── app.go      # top-level Bubble Tea model
│       ├── views/
│       │   ├── main.go       # main view (playbooks + inventory + options)
│       │   ├── run.go        # run view (task list + output)
│       │   ├── picker.go     # project picker
│       │   └── host.go       # host detail overlay
│       └── styles/     # Lip Gloss style definitions
├── go.mod
├── go.sum
├── CLAUDE.md
└── README.md
```

---

## MVP Scope (v0.1)

- [ ] Config loading (single project, defaults block only)
- [ ] Playbook list from directory scan
- [ ] Inventory tree from `ansible-inventory --list`
- [ ] Run options form (limit, tags, extra-vars, check, diff)
- [ ] Subprocess runner with stdout streaming
- [ ] JSON callback output parsing and task list update
- [ ] Output panel with scroll
- [ ] Run cancel via Esc
- [ ] Final stats in status bar

## v0.2

- [ ] Multi-project support + project picker
- [ ] Host detail overlay
- [ ] Output filter by hostname
- [ ] Save output to file
- [ ] Inventory refresh (`r`)
- [ ] Vault password file awareness

## v0.3

- [ ] Tags autocomplete from playbook `--list-tags`
- [ ] Host/group autocomplete in limit field
- [ ] Run history (last N runs with timestamp + outcome)
- [ ] Config file UI (edit path fields in-TUI)

---

## CLAUDE.md (starter)

```markdown
# anvil

TUI Ansible runner in Go using Bubble Tea.

## Build & test
- `go build ./...` to compile
- `go test ./...` to run tests
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
- JSON stdout callback is set via env var, not ansible.cfg
```
