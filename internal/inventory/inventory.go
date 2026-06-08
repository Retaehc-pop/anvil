package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
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

// rawInventory matches the shape of ansible-inventory --list output.
type rawInventory struct {
	Meta struct {
		HostVars map[string]map[string]any `json:"hostvars"`
	} `json:"_meta"`
	All rawGroup `json:"all"`
	// remaining keys are group names
	Groups map[string]rawGroup
}

type rawGroup struct {
	Hosts    []string          `json:"hosts"`
	Children []string          `json:"children"`
	Vars     map[string]string `json:"vars"`
}

func Load(ctx context.Context, inventoryPath string) (*Tree, error) {
	out, err := exec.CommandContext(ctx, "ansible-inventory", "-i", inventoryPath, "--list").Output()
	if err != nil {
		return nil, fmt.Errorf("ansible-inventory: %w", err)
	}
	return parse(out)
}

// FetchHostVars runs ansible-inventory --host <name> and populates node.Vars.
func FetchHostVars(ctx context.Context, inventoryPath, hostname string) (map[string]any, error) {
	out, err := exec.CommandContext(ctx, "ansible-inventory", "-i", inventoryPath, "--host", hostname).Output()
	if err != nil {
		return nil, fmt.Errorf("ansible-inventory --host: %w", err)
	}
	var vars map[string]any
	if err := json.Unmarshal(out, &vars); err != nil {
		return nil, fmt.Errorf("parse host vars: %w", err)
	}
	return vars, nil
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
