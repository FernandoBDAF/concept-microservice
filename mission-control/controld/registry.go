package main

// registry.go — the systems registry loader (HANDOFF §1).
//
// It parses systems/*.yaml into []System (the type in actions.go), validates
// every file against schema v0, and doubles as the action whitelist: nothing
// outside the loaded registry is ever exec'd. The repo-committed YAML files
// are the trust boundary — commands are NOT required to start with "make"
// (hello-guest legitimately uses kubectl); the registry content IS the
// whitelist.

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"

	"log/slog"

	"gopkg.in/yaml.v3"
)

// validTargets is the closed set of target keys schema v0 allows.
var validTargets = map[string]bool{"compose": true, "kind": true, "aws": true}

// kebabRe validates system names: lowercase, digits, single dashes.
var kebabRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Registry is the loaded, validated set of systems. Safe for concurrent reads;
// Reload swaps the snapshot under a write lock.
type Registry struct {
	dir string
	log *slog.Logger

	mu      sync.RWMutex
	byName  map[string]System
	ordered []System // sorted by name
}

// LoadRegistry parses and validates every systems/*.yaml under dir. A single
// invalid file fails the whole load (startup-fatal by design).
func LoadRegistry(dir string, log *slog.Logger) (*Registry, error) {
	if log == nil {
		log = slog.Default()
	}
	r := &Registry{dir: dir, log: log}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// Reload re-parses the directory and atomically swaps the snapshot. On any
// error the previous snapshot is left untouched.
func (r *Registry) Reload() error {
	byName, ordered, err := loadDir(r.dir)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.byName = byName
	r.ordered = ordered
	r.mu.Unlock()
	r.log.Info("registry loaded", "dir", r.dir, "systems", len(ordered))
	return nil
}

func loadDir(dir string) (map[string]System, []System, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, nil, fmt.Errorf("registry: glob %s: %w", dir, err)
	}
	sort.Strings(paths)

	byName := make(map[string]System, len(paths))
	for _, p := range paths {
		sys, err := parseSystemFile(p)
		if err != nil {
			return nil, nil, err
		}
		if _, dup := byName[sys.Name]; dup {
			return nil, nil, fmt.Errorf("registry: duplicate system name %q (%s)", sys.Name, filepath.Base(p))
		}
		byName[sys.Name] = sys
	}

	ordered := make([]System, 0, len(byName))
	for _, s := range byName {
		ordered = append(ordered, s)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	return byName, ordered, nil
}

func parseSystemFile(path string) (System, error) {
	f, err := os.Open(path)
	if err != nil {
		return System{}, fmt.Errorf("registry: open %s: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true) // unknown top-level (and nested struct) keys are errors
	var sys System
	if err := dec.Decode(&sys); err != nil {
		return System{}, fmt.Errorf("registry: parse %s: %w", filepath.Base(path), err)
	}
	if err := validateSystem(sys, filepath.Base(path)); err != nil {
		return System{}, err
	}
	return sys, nil
}

func validateSystem(sys System, file string) error {
	where := func(msg string) error { return fmt.Errorf("registry: %s: %s", file, msg) }

	if strings.TrimSpace(sys.Name) == "" {
		return where("empty system name")
	}
	if !kebabRe.MatchString(sys.Name) {
		return where(fmt.Sprintf("name %q is not kebab-case", sys.Name))
	}
	if len(sys.Targets) == 0 {
		return where("no targets declared (need at least one of compose|kind|aws)")
	}
	for key, tc := range sys.Targets {
		if !validTargets[key] {
			return where(fmt.Sprintf("unknown target key %q (allowed: compose|kind|aws)", key))
		}
		if strings.TrimSpace(tc.Up) == "" {
			return where(fmt.Sprintf("target %q missing up command", key))
		}
		if strings.TrimSpace(tc.Down) == "" {
			return where(fmt.Sprintf("target %q missing down command", key))
		}
		if strings.TrimSpace(tc.Status) == "" {
			return where(fmt.Sprintf("target %q missing status command", key))
		}
	}
	for i, s := range sys.Scale {
		if strings.TrimSpace(s.Component) == "" {
			return where(fmt.Sprintf("scale[%d] has empty component", i))
		}
		if strings.TrimSpace(s.Compose) == "" && strings.TrimSpace(s.Kind) == "" {
			return where(fmt.Sprintf("scale component %q has no per-target template", s.Component))
		}
	}
	return nil
}

// System returns the named system.
func (r *Registry) System(name string) (System, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byName[name]
	return s, ok
}

// Systems returns all systems sorted by name (a fresh slice).
func (r *Registry) Systems() []System {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]System, len(r.ordered))
	copy(out, r.ordered)
	return out
}

// StartSIGHUPReload reloads the registry on every SIGHUP until ctx is done.
// A reload error is logged and the previous snapshot is kept.
func (r *Registry) StartSIGHUPReload(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		defer signal.Stop(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				if err := r.Reload(); err != nil {
					r.log.Error("registry reload failed (keeping previous)", "error", err.Error())
				}
			}
		}
	}()
}
