package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Version  int      `toml:"version"`
	Shared   []string `toml:"shared"`
	Isolated []string `toml:"isolated"`
	Export   Export   `toml:"export"`
	Launch   Launch   `toml:"launch"`
}

type Export struct {
	Exclude []string `toml:"exclude"`
}

type Launch struct {
	Command []string `toml:"command"`
}

const supportedVersion = 2

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := toml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Version == 0 {
		c.Version = supportedVersion
	}
	if c.Version > supportedVersion {
		return Config{}, fmt.Errorf("config version %d is newer than this ccs (%d)", c.Version, supportedVersion)
	}
	if len(c.Shared) == 0 && len(c.Isolated) == 0 {
		d := Default()
		c.Shared = d.Shared
		c.Isolated = d.Isolated
		if len(c.Export.Exclude) == 0 {
			c.Export = d.Export
		}
	}
	return c, nil
}

func Save(path string, c Config) error {
	if c.Version == 0 {
		c.Version = supportedVersion
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}
