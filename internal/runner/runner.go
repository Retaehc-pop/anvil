package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Retaehc-pop/anvil/internal/config"
	"github.com/Retaehc-pop/anvil/internal/parser"
)

// Options mirrors the Run Options form in the UI.
type Options struct {
	Playbook  string
	Limit     string
	Tags      string
	ExtraVars string
	Check     bool
	Diff      bool
	Format    parser.Format
}

// LineMsg carries a parsed event back to the Bubble Tea update loop.
type LineMsg struct {
	Event  parser.Event
	Stderr bool
}

// Run spawns ansible-playbook and sends parsed events on ch until the process
// exits or ctx is cancelled. The channel is closed when Run returns.
func Run(ctx context.Context, proj config.Project, opts Options, ch chan<- LineMsg) error {
	defer close(ch)

	args := buildArgs(proj, opts)
	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)

	callbackVal := "json"
	if opts.Format == parser.FormatYAML {
		callbackVal = "yaml"
	}
	cmd.Env = setEnv(inheritEnv(), "ANSIBLE_STDOUT_CALLBACK", callbackVal)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ansible-playbook: %w", err)
	}

	// Stream stdout
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			ev := parser.Parse(sc.Text(), opts.Format)
			ch <- LineMsg{Event: ev}
		}
	}()

	// Stream stderr
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			ch <- LineMsg{
				Event:  parser.Event{Type: parser.EventRaw, Raw: sc.Text()},
				Stderr: true,
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ansible-playbook: %w", err)
	}
	return nil
}

func buildArgs(proj config.Project, opts Options) []string {
	args := []string{"-i", proj.Inventory}

	if opts.Limit != "" {
		args = append(args, "--limit", opts.Limit)
	}
	if opts.Tags != "" {
		args = append(args, "--tags", opts.Tags)
	}
	if opts.ExtraVars != "" {
		args = append(args, "--extra-vars", opts.ExtraVars)
	}
	if opts.Check {
		args = append(args, "--check")
	}
	if opts.Diff {
		args = append(args, "--diff")
	}
	if proj.VaultPasswordFile != "" {
		args = append(args, "--vault-password-file", proj.VaultPasswordFile)
	}

	args = append(args, opts.Playbook)
	return args
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return append(out, prefix+value)
}

func inheritEnv() []string {
	// Pass through the parent environment so PATH, SSH_AUTH_SOCK, etc. reach ansible.
	return os.Environ()
}
