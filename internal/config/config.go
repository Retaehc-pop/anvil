package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Defaults struct {
	Inventory         string `toml:"inventory"`
	PlaybookDir       string `toml:"playbook_dir"`
	VaultPasswordFile string `toml:"vault_password_file"`
	CommandPrefix     string `toml:"command_prefix"`
}

type Project struct {
	Name              string `toml:"name"`
	Inventory         string `toml:"inventory"`
	PlaybookDir       string `toml:"playbook_dir"`
	VaultPasswordFile string `toml:"vault_password_file"`
	CommandPrefix     string `toml:"command_prefix"`
}

type Config struct {
	Defaults Defaults  `toml:"defaults"`
	Projects []Project `toml:"project"`
}

func Load() (*Config, error) {
	path := configPath()
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func configPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "anvil", "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "anvil", "config.toml")
}

// ActiveProjects returns the list of selectable projects. If no [[project]]
// entries exist, a synthetic one is built from [defaults].
func (c *Config) ActiveProjects() []Project {
	if len(c.Projects) > 0 {
		return c.Projects
	}
	return []Project{{
		Name:              "default",
		Inventory:         c.Defaults.Inventory,
		PlaybookDir:       c.Defaults.PlaybookDir,
		VaultPasswordFile: c.Defaults.VaultPasswordFile,
		CommandPrefix:     c.Defaults.CommandPrefix,
	}}
}
