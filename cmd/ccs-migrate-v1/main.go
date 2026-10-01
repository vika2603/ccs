// Command ccs-migrate-v1 converts a ~/.ccs tree from the classification-based
// layout (config.toml, shared/, profiles/<name>/, env/) to profile files on
// top of ~/.claude. It is a one-off tool.
//
// Every old profile directory is copied to accounts/<name> as a login
// profile, so its history, projects, and login carry over; its stored
// keychain login is copied too. Shared assets from ~/.ccs/shared that
// ~/.claude lacks are copied into it; existing ~/.claude entries are never
// replaced. The old tree is left in place and ignored by the new ccs, so
// sessions still running on it keep working; delete it once the new layout
// works.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fsutil"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "print the plan without changing anything")
	flag.Parse()
	home, err := os.UserHomeDir()
	if err == nil {
		err = migrate(home, *dryRun, creds.New())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ccs-migrate-v1:", err)
		os.Exit(1)
	}
}

type oldConfig struct {
	Shared []string `toml:"shared"`
}

type step struct {
	desc string
	run  func() error
}

func migrate(home string, dryRun bool, store creds.Store) error {
	p := layout.New(home)
	root := p.Root()
	var cfg oldConfig
	if _, err := toml.DecodeFile(filepath.Join(root, "config.toml"), &cfg); err != nil {
		return fmt.Errorf("read old config.toml (is this an old ccs tree?): %w", err)
	}
	if _, err := os.Stat(p.AccountsDir()); err == nil {
		return fmt.Errorf("%s already exists; the tree looks migrated", p.AccountsDir())
	}

	var steps []step
	add := func(desc string, run func() error) { steps = append(steps, step{desc, run}) }
	var notes []string

	oldShared := filepath.Join(root, "shared")
	for _, name := range cfg.Shared {
		src := filepath.Join(oldShared, name)
		if _, err := os.Lstat(src); err != nil {
			continue
		}
		added, differ, err := planMerge(src, filepath.Join(p.ClaudeDir(), name), "~/.claude/"+name)
		if err != nil {
			return err
		}
		for _, d := range differ {
			notes = append(notes, d+" differs from ~/.ccs/shared and is kept as is")
		}
		for _, a := range added {
			add("copy ~/.ccs/shared/"+strings.TrimPrefix(a.rel, "~/.claude/")+" to "+a.rel, func() error {
				if err := os.MkdirAll(filepath.Dir(a.dst), 0o755); err != nil {
					return err
				}
				return copyPreserving(a.src, a.dst)
			})
		}
	}
	if entries, err := os.ReadDir(oldShared); err == nil {
		for _, e := range entries {
			if !slices.Contains(cfg.Shared, e.Name()) {
				notes = append(notes, "~/.ccs/shared/"+e.Name()+" is not in the old shared list and is not copied")
			}
		}
	}

	// A forked settings.json is diffed against ~/.claude/settings.json, which
	// every profile inherits after the migration.
	var baseSettings map[string]any
	_ = readJSON(filepath.Join(p.ClaudeDir(), "settings.json"), &baseSettings)
	if baseSettings == nil {
		_ = readJSON(filepath.Join(oldShared, "settings.json"), &baseSettings)
	}

	oldProfiles := filepath.Join(root, "profiles")
	entries, err := os.ReadDir(oldProfiles)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		oldDir := filepath.Join(oldProfiles, name)
		if !e.IsDir() || layout.ValidName(name) != nil {
			notes = append(notes, "skipped "+oldDir+": not a valid profile directory")
			continue
		}
		acct := p.AccountDir(name)
		settings, isolateSettings, note := profileSettings(root, oldDir, name, baseSettings)
		if note != "" {
			notes = append(notes, note)
		}

		add(fmt.Sprintf("copy profile %s to %s with its stored login", name, acct), func() error {
			if err := os.MkdirAll(p.AccountsDir(), 0o700); err != nil {
				return err
			}
			if err := copyPreserving(oldDir, acct); err != nil {
				return err
			}
			return copyLogin(store, oldDir, acct)
		})
		add("drop "+name+"'s links into ~/.ccs/shared (relinked to ~/.claude on next launch)", func() error {
			return dropSharedLinks(acct, oldShared, name, isolateSettings)
		})
		add("write "+p.ProfileFile(name), func() error {
			return writeProfile(p, name, settings, isolateSettings)
		})
	}

	for i, s := range steps {
		fmt.Printf("%2d. %s\n", i+1, s.desc)
	}
	for _, n := range notes {
		fmt.Println("note:", n)
	}
	if dryRun {
		fmt.Println("dry run: nothing changed")
		return nil
	}
	for i, s := range steps {
		if err := s.run(); err != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, s.desc, err)
		}
	}
	fmt.Println("done; the old tree is untouched. Once the new layout works, remove it with:")
	old := []string{filepath.Join(root, "config.toml"), oldShared, filepath.Join(root, "env")}
	for _, e := range entries {
		if e.IsDir() {
			old = append(old, filepath.Join(oldProfiles, e.Name()))
		}
	}
	fmt.Println("  rm -rf " + strings.Join(old, " "))
	return nil
}

// profileSettings builds the [settings] of an old profile from its env file
// and a forked settings.json. A fork is folded into [settings] only when it
// differs from the shared settings in env entries or plain top-level values;
// other differences (lists such as permissions and hooks, which Claude Code
// would merge rather than replace) keep the fork as an isolated file.
func profileSettings(root, oldDir, name string, base map[string]any) (map[string]any, bool, string) {
	settings := map[string]any{}
	env := map[string]any{}
	var envFile struct {
		Env map[string]string `toml:"env"`
	}
	if _, err := toml.DecodeFile(filepath.Join(root, "env", name+".toml"), &envFile); err == nil {
		for k, v := range envFile.Env {
			env[k] = v
		}
	}

	forkPath := filepath.Join(oldDir, "settings.json")
	info, err := os.Lstat(forkPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		if len(env) > 0 {
			settings["env"] = env
		}
		return settings, false, ""
	}
	var fork map[string]any
	if err := readJSON(forkPath, &fork); err != nil {
		return settingsWithEnv(settings, env), true, name + ": settings.json could not be parsed and stays as the profile's own file"
	}
	plain := map[string]any{}
	complexDiff := false
	for k, v := range fork {
		if k == "env" {
			forkEnv, _ := v.(map[string]any)
			baseEnv, _ := base["env"].(map[string]any)
			for ek, ev := range forkEnv {
				if !reflect.DeepEqual(baseEnv[ek], ev) {
					env[ek] = ev
				}
			}
			continue
		}
		if reflect.DeepEqual(base[k], v) {
			continue
		}
		switch v.(type) {
		case string, bool, float64, nil:
			plain[k] = v
		default:
			complexDiff = true
		}
	}
	var missing []string
	for k := range base {
		if _, ok := fork[k]; !ok {
			missing = append(missing, k)
		}
	}
	if complexDiff || len(missing) > 0 {
		reason := "differs in lists or objects"
		if len(missing) > 0 {
			slices.Sort(missing)
			reason = "drops shared keys " + strings.Join(missing, ", ")
		}
		return settingsWithEnv(settings, env), true, name + ": settings.json " + reason + ", so it stays as the profile's own file (isolate = [\"settings.json\"])"
	}
	maps.Copy(settings, plain)
	return settingsWithEnv(settings, env), false, ""
}

func settingsWithEnv(settings, env map[string]any) map[string]any {
	if len(env) > 0 {
		settings["env"] = env
	}
	return settings
}

// dropSharedLinks removes links into the old shared directory, and a forked
// settings.json that was folded into [settings], so Sync relinks them to
// ~/.claude.
func dropSharedLinks(acct, oldShared, name string, keepSettings bool) error {
	entries, err := os.ReadDir(acct)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(acct, e.Name())
		if target, err := os.Readlink(path); err == nil {
			if strings.HasPrefix(target, oldShared+string(os.PathSeparator)) {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
			continue
		}
		if e.Name() == "settings.json" && !keepSettings {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeProfile(p layout.Paths, name string, settings map[string]any, isolateSettings bool) error {
	pr := struct {
		Login    bool           `toml:"login"`
		Isolate  []string       `toml:"isolate,omitempty"`
		Settings map[string]any `toml:"settings,omitempty"`
	}{Login: true, Settings: settings}
	if isolateSettings {
		pr.Isolate = []string{"settings.json"}
	}
	var b strings.Builder
	b.WriteString("# Migrated from the old ccs layout. Edit with: ccs edit " + name + "\n")
	if err := toml.NewEncoder(&b).Encode(pr); err != nil {
		return err
	}
	if _, err := profile.Parse([]byte(b.String())); err != nil {
		return fmt.Errorf("generated profile is invalid: %w", err)
	}
	return fsutil.WriteFileAtomic(p.ProfileFile(name), []byte(b.String()), 0o600)
}

// copyLogin copies the stored login of oldDir to newDir and keeps the old
// one, so a session still running on oldDir stays logged in.
func copyLogin(s creds.Store, oldDir, newDir string) error {
	data, err := s.Read(oldDir)
	if errors.Is(err, creds.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.Write(newDir, data); err != nil {
		return err
	}
	if got, err := s.Read(newDir); err != nil || !bytes.Equal(got, data) {
		return fmt.Errorf("stored login for %s did not read back intact", newDir)
	}
	return nil
}

type copyOp struct{ src, dst, rel string }

// planMerge lists what to copy from src into dst without replacing anything
// already in dst: entries missing from dst are copied, directories are merged
// entry by entry, and files whose content differs are reported. Existing
// ~/.claude entries are kept because they may be links into a dotfiles
// repository or newer than the old shared copy.
func planMerge(src, dst, rel string) (added []copyOp, differ []string, err error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, nil, err
	}
	dstInfo, err := os.Stat(dst)
	if errors.Is(err, os.ErrNotExist) {
		if _, lerr := os.Lstat(dst); lerr == nil {
			return nil, []string{rel + " (dangling link)"}, nil
		}
		return []copyOp{{src, dst, rel}}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if srcInfo.IsDir() != dstInfo.IsDir() {
		return nil, []string{rel}, nil
	}
	if !srcInfo.IsDir() {
		a, err1 := os.ReadFile(src)
		b, err2 := os.ReadFile(dst)
		if err1 != nil || err2 != nil || !bytes.Equal(a, b) {
			differ = append(differ, rel)
		}
		return nil, differ, nil
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.Name() == ".DS_Store" {
			continue
		}
		a, d, err := planMerge(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), rel+"/"+e.Name())
		if err != nil {
			return nil, nil, err
		}
		added, differ = append(added, a...), append(differ, d...)
	}
	return added, differ, nil
}

// copyPreserving copies src to dst keeping symlinks, modes, and timestamps.
func copyPreserving(src, dst string) error {
	if out, err := exec.Command("cp", "-a", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("cp -a %s %s: %w: %s", src, dst, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
