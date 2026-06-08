# anvil

A terminal UI for Ansible, written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Run playbooks interactively, browse your inventory, tune `--limit`/`--tags`/`--extra-vars`, preview host variables, and watch task execution in real time — without ever typing a long `ansible-playbook` command by hand.

## Requirements

- Go 1.21+
- `ansible` and `ansible-playbook` in `PATH`

## Installation

```sh
go install github.com/Retaehc-pop/anvil/cmd/anvil@latest
```

Or build from source:

```sh
git clone https://github.com/Retaehc-pop/anvil
cd anvil
go build -o bin/anvil ./cmd/anvil
```

## Usage

```
anvil [flags]

Flags:
  -h, --help     Show help
  -v, --version  Print version
```

Just run `anvil` — it reads your config and opens the TUI.

## Configuration

`~/.config/anvil/config.toml` (respects `$XDG_CONFIG_HOME`)

```toml
[defaults]
inventory          = "~/infra/inventory"
playbook_dir       = "~/infra/playbooks"
vault_password_file = "~/.vault_pass"

[[project]]
name               = "hpc-cluster"
inventory          = "/opt/cluster/inventory"
playbook_dir       = "/opt/cluster/playbooks"
vault_password_file = "/opt/cluster/.vault_pass"

[[project]]
name               = "monitoring"
inventory          = "/opt/monitoring/inventory"
playbook_dir       = "/opt/monitoring/playbooks"
```

If no `[[project]]` entries exist, `[defaults]` is used as a single unnamed project. If multiple projects are defined, a picker is shown on startup.

See `example/` for a minimal working layout.

## Views

### Main view

Two panes (Tab to switch focus):

- **Left — Playbook list**: scans `playbook_dir` for `*.yml`/`*.yaml` files.
- **Right — Inventory tree**: populated from `ansible-inventory --list`; expand/collapse groups with Space; press Enter on a host/group to set it as the limit.
- **Bottom — Run options**: Limit, Tags, Extra Vars, Check mode, Diff mode.

| Key | Action |
|-----|--------|
| `Tab` | Switch pane focus |
| `Enter` | Open preview |
| `l` | Focus Limit field |
| `t` | Focus Tags field |
| `e` | Focus Extra Vars field |
| `c` | Toggle check mode |
| `d` | Toggle diff mode |
| `?` | Toggle help overlay |
| `q` | Quit |

### Preview view

Shows the variables that will apply to each matched host before running.

| Key | Action |
|-----|--------|
| `r` | Run |
| `d` | Dry run (forces `--check`) |
| `Esc` | Back to main view (options preserved) |

### Run view

Live task execution with a task list, per-host progress bars, and a scrollable output panel.

| Key | Action |
|-----|--------|
| `Esc` | Cancel run (SIGINT) |
| `Tab` | Switch focus panel |
| `f` | Filter output by hostname |
| `s` | Save output to file |
| `PgUp`/`PgDn` | Scroll output |
| `g` / `G` | Jump to top / bottom of output |

## How it works

`anvil` wraps `ansible-playbook` with `ANSIBLE_STDOUT_CALLBACK=json` and reads its stdout line by line. Each JSON event is classified and mapped onto the TUI in real time. No Ansible Python API is used — pure subprocess.

The run command anvil constructs:

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

## License

MIT
