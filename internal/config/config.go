package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Version int    `toml:"version"`
	Fields  Fields `toml:"fields"`
}

type Fields struct {
	Shared    []string `toml:"shared"`
	Isolated  []string `toml:"isolated"`
	Transient []string `toml:"transient"`
}

const supportedVersion = 1

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
	if len(c.Fields.Shared) == 0 && len(c.Fields.Isolated) == 0 && len(c.Fields.Transient) == 0 {
		c.Fields = Default().Fields
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
