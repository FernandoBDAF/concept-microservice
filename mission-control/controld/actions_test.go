package main

import (
	"testing"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := LoadRegistry("testdata/registry", quietLogger())
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return reg
}

func TestResolveCommandHappyPaths(t *testing.T) {
	reg := testRegistry(t)
	awsOn := Config{EnableAWS: true}
	noAWS := Config{}

	cases := []struct {
		name string
		cfg  Config
		req  ActionRequest
		want string
	}{
		{"up compose", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "up"}, "make up"},
		{"status kind", noAWS, ActionRequest{System: "lab", Target: "kind", Verb: "status"}, "make cluster-status"},
		{"down with confirm", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "down", Params: map[string]string{"confirm": "true"}}, "make down"},
		{"scale compose", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "3"}}, "make scale S=email-worker N=3"},
		{"scale kind", noAWS, ActionRequest{System: "lab", Target: "kind", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "10"}}, "make cluster-scale S=email-worker N=10"},
		{"experiment", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "experiment", Params: map[string]string{"id": "exp-42-worker-scale"}}, "make experiment E=exp-42-worker-scale"},
		{"aws up enabled", awsOn, ActionRequest{System: "lab", Target: "aws", Verb: "up"}, "make aws-up"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveCommand(reg, c.cfg, c.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveCommandRejections(t *testing.T) {
	reg := testRegistry(t)
	noAWS := Config{}

	cases := []struct {
		name       string
		cfg        Config
		req        ActionRequest
		wantStatus int
	}{
		{"unknown system", noAWS, ActionRequest{System: "nope", Target: "compose", Verb: "up"}, 404},
		{"unknown verb", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "nuke"}, 400},
		{"target not in system", noAWS, ActionRequest{System: "hello-guest", Target: "aws", Verb: "up"}, 403}, // aws gated first
		{"target missing (kind on system w/o it)", noAWS, ActionRequest{System: "lab", Target: "bogus", Verb: "up"}, 404},
		{"down without confirm", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "down"}, 400},
		{"down wrong confirm", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "down", Params: map[string]string{"confirm": "yes"}}, 400},
		{"scale n=0", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "0"}}, 400},
		{"scale n=11", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "11"}}, 400},
		{"scale n=abc", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "abc"}}, 400},
		{"scale injection in n", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "3; rm -rf /"}}, 400},
		{"scale unknown component", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "ghost", "n": "2"}}, 400},
		{"scale injection in component", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "scale", Params: map[string]string{"component": "email-worker; touch pwned", "n": "2"}}, 400},
		{"scale on aws (no template, enabled)", Config{EnableAWS: true}, ActionRequest{System: "lab", Target: "aws", Verb: "scale", Params: map[string]string{"component": "email-worker", "n": "2"}}, 400},
		{"experiment bad id", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "experiment", Params: map[string]string{"id": "exp-bad; rm -rf /"}}, 400},
		{"experiment uppercase id", noAWS, ActionRequest{System: "lab", Target: "compose", Verb: "experiment", Params: map[string]string{"id": "EXP-42"}}, 400},
		{"experiment unknown target", noAWS, ActionRequest{System: "lab", Target: "anything", Verb: "experiment", Params: map[string]string{"id": "exp-1"}}, 404},
		{"experiment empty target", noAWS, ActionRequest{System: "lab", Verb: "experiment", Params: map[string]string{"id": "exp-1"}}, 404},
		{"aws disabled", noAWS, ActionRequest{System: "lab", Target: "aws", Verb: "up"}, 403},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := resolveCommand(reg, c.cfg, c.req)
			if err == nil {
				t.Fatalf("expected error")
			}
			if got := statusOf(err); got != c.wantStatus {
				t.Errorf("status = %d, want %d (%v)", got, c.wantStatus, err)
			}
		})
	}
}

func TestResolveExperimentWithoutExperiments(t *testing.T) {
	reg, err := LoadRegistry("testdata/no-exp", quietLogger())
	if err != nil {
		t.Fatalf("load no-exp registry: %v", err)
	}
	_, err = resolveCommand(reg, Config{}, ActionRequest{
		System: "plain", Target: "compose", Verb: "experiment",
		Params: map[string]string{"id": "exp-1"},
	})
	if err == nil {
		t.Fatal("expected error: system declares no experiments")
	}
	if statusOf(err) != 400 {
		t.Errorf("status = %d, want 400", statusOf(err))
	}
}

func TestParseScaleN(t *testing.T) {
	good := map[string]int{"1": 1, "10": 10, "5": 5}
	for in, want := range good {
		if got, err := parseScaleN(in); err != nil || got != want {
			t.Errorf("parseScaleN(%q) = %d,%v want %d", in, got, err, want)
		}
	}
	bad := []string{"0", "11", "-1", "abc", "", "3.5", "3;rm", "1e2"}
	for _, in := range bad {
		if _, err := parseScaleN(in); err == nil {
			t.Errorf("parseScaleN(%q) should error", in)
		}
	}
}
