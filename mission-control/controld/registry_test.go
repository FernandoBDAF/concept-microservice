package main

import (
	"io"
	"log/slog"
	"testing"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoadRegistryGood(t *testing.T) {
	reg, err := LoadRegistry("testdata/registry", quietLogger())
	if err != nil {
		t.Fatalf("load good registry: %v", err)
	}
	got := reg.Systems()
	if len(got) != 2 {
		t.Fatalf("want 2 systems, got %d", len(got))
	}
	// Sorted by name: hello-guest before lab.
	if got[0].Name != "hello-guest" || got[1].Name != "lab" {
		t.Fatalf("want [hello-guest lab], got [%s %s]", got[0].Name, got[1].Name)
	}
	lab, ok := reg.System("lab")
	if !ok {
		t.Fatal("lab not found")
	}
	if lab.Targets["compose"].Up != "make up" {
		t.Errorf("lab compose up = %q", lab.Targets["compose"].Up)
	}
	if len(lab.Scale) != 3 {
		t.Errorf("lab scale entries = %d, want 3", len(lab.Scale))
	}
	// hello-guest legitimately uses kubectl, not make — the registry is the
	// whitelist, not a make-only rule.
	hg, _ := reg.System("hello-guest")
	if hg.Targets["kind"].Up != "kubectl apply -k guests/hello-guest/k8s/base" {
		t.Errorf("hello-guest kind up = %q", hg.Targets["kind"].Up)
	}
}

func TestLoadRegistryBroken(t *testing.T) {
	cases := map[string]string{
		"unknown target key": "testdata/broken-unknown-target",
		"missing verb":       "testdata/broken-missing-verb",
		"duplicate name":     "testdata/broken-dup",
		"unknown top key":    "testdata/broken-unknown-key",
		"non-kebab name":     "testdata/broken-noncabeb",
		"empty scale tmpl":   "testdata/broken-empty-scale",
	}
	for name, dir := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadRegistry(dir, quietLogger()); err == nil {
				t.Fatalf("expected load error for %s (%s)", name, dir)
			}
		})
	}
}

func TestRegistryReload(t *testing.T) {
	reg, err := LoadRegistry("testdata/registry", quietLogger())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := reg.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reg.Systems()) != 2 {
		t.Fatalf("after reload want 2 systems, got %d", len(reg.Systems()))
	}
}
