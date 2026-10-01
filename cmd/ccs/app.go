package main

import (
	"errors"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/doctor"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
)

// app holds what every command needs: the ~/.ccs paths, the loaded
// config.toml with its field registry, and a profile manager bound to the
// platform credential store.
type app struct {
	layout.Paths
	cfg   config.Config
	reg   *fields.Registry
	creds creds.Store
	mgr   profile.Manager
}

// loadPaths returns an app with paths and the credential store only. Use it
// for commands that must work even when config.toml cannot be loaded.
func loadPaths() (app, error) {
	p, err := layout.FromEnv()
	if err != nil {
		return app{}, err
	}
	return app{Paths: p, creds: creds.New()}, nil
}

func loadApp() (app, error) {
	p, err := layout.FromEnv()
	if err != nil {
		return app{}, err
	}
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return app{}, err
	}
	reg := fields.NewRegistry(cfg)
	store := creds.New()
	return app{
		Paths: p,
		cfg:   cfg,
		reg:   reg,
		creds: store,
		mgr:   profile.NewManager(p, reg).WithCreds(store),
	}, nil
}

func (a app) ops() fields.Ops { return fields.NewOps(a.Paths, a.reg) }

func (a app) checker() doctor.Checker {
	defaults := fields.NewRegistry(config.Default())
	return doctor.NewChecker(a.Paths, a.reg, defaults, creds.ListServices, creds.DefaultClaudeDir())
}

// profileOrActive returns name when it is non-empty, otherwise the active
// profile.
func (a app) profileOrActive(name string) (string, error) {
	if name != "" {
		return name, nil
	}
	active, err := a.Active()
	if err != nil {
		return "", err
	}
	if active == "" {
		return "", errors.New("no profile given and no active profile; pass <profile> or run `ccs use` first")
	}
	return active, nil
}
