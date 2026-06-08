package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/Retaehc-pop/anvil/internal/config"
)

// Node is a single item in the inventory tree (group or host).
type Node struct {
	Name     string
	IsGroup  bool
	Children []*Node
	Vars     map[string]any // populated lazily via FetchHostVars
}

// Tree is the parsed result of `ansible-inventory --list`.
type Tree struct {
	Root  *Node
	Hosts map[string]*Node // flat lookup by hostname
}

type rawGroup struct {
	Hosts    []string          `json:"hosts"`
	Children []string          `json:"children"`
	Vars     map[string]string `json:"vars"`
}

func Load(ctx context.Context, proj config.Project) (*Tree, error) {
	cmd := prefixedCmd(ctx, proj.CommandPrefix, "ansible-inventory", "-i", proj.Inventory, "--list")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ansible-inventory: %w", err)
	}
	return parse(out)
}

// FetchHostVars runs ansible-inventory --host <name> and returns its variables.
func FetchHostVars(ctx context.Context, proj config.Project, hostname string) (map[string]any, error) {
	cmd := prefixedCmd(ctx, proj.CommandPrefix, "ansible-inventory", "-i", proj.Inventory, "--host", hostname)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ansible-inventory --host: %w", err)
	}
	var vars map[string]any
	if err := json.Unmarshal(out, &vars); err != nil {
		return nil, fmt.Errorf("parse host vars: %w", err)
	}
	return vars, nil
}

// prefixedCmd builds an exec.Cmd, prepending any command_prefix words before
// the ansible executable. e.g. prefix="rex" → exec("rex", "ansible-inventory", ...)
func prefixedCmd(ctx context.Context, prefix, executable string, args ...string) *exec.Cmd {
	if prefix == "" {
		return exec.CommandContext(ctx, executable, args...)
	}
	parts := strings.Fields(prefix)
	all := append(parts[1:], executable)
	all = append(all, args...)
	return exec.CommandContext(ctx, parts[0], all...)
}

func parse(data []byte) (*Tree, error) {
	// Unmarshal into a raw map first so we can iterate all group keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse inventory: %w", err)
	}

	groups := map[string]rawGroup{}
	var hostVars map[string]map[string]any

	for key, val := range raw {
		if key == "_meta" {
			var meta struct {
				HostVars map[string]map[string]any `json:"hostvars"`
			}
			if err := json.Unmarshal(val, &meta); err == nil {
				hostVars = meta.HostVars
			}
			continue
		}
		var g rawGroup
		if err := json.Unmarshal(val, &g); err == nil {
			groups[key] = g
		}
	}

	hosts := map[string]*Node{}
	tree := &Tree{Hosts: hosts}
	tree.Root = buildNode("all", groups, hosts, hostVars, map[string]bool{})
	return tree, nil
}

func buildNode(name string, groups map[string]rawGroup, hosts map[string]*Node, hostVars map[string]map[string]any, visited map[string]bool) *Node {
	node := &Node{Name: name, IsGroup: true}
	if visited[name] {
		return node
	}
	visited[name] = true

	g, ok := groups[name]
	if !ok {
		return node
	}

	childNames := append([]string{}, g.Children...)
	sort.Strings(childNames)
	for _, child := range childNames {
		node.Children = append(node.Children, buildNode(child, groups, hosts, hostVars, visited))
	}

	hostNames := append([]string{}, g.Hosts...)
	sort.Strings(hostNames)
	for _, h := range hostNames {
		if _, exists := hosts[h]; !exists {
			hn := &Node{Name: h, IsGroup: false}
			if hostVars != nil {
				hn.Vars = hostVars[h]
			}
			hosts[h] = hn
		}
		node.Children = append(node.Children, hosts[h])
	}

	return node
}
