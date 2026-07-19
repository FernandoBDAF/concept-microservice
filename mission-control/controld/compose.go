package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// composePSEntry mirrors the fields we care about from
// `docker compose -p microservices ps --format json`, which emits one JSON
// object per line.
type composePSEntry struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
	Image   string `json:"Image"`
}

// ComposeService is the summarized per-service status for the compose target.
type ComposeService struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Health string `json:"health"`
	Image  string `json:"image"`
}

// summarizeComposePS parses the line-delimited JSON output of
// `docker compose ps --format json` into per-service summaries.
// It tolerates (and skips) blank lines. Some docker versions emit a single
// JSON array instead of one object per line; both forms are handled.
func summarizeComposePS(raw []byte) ([]ComposeService, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return []ComposeService{}, nil
	}

	var entries []composePSEntry
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			return nil, fmt.Errorf("parse compose ps array: %w", err)
		}
	} else {
		scanner := bufio.NewScanner(bytes.NewReader(trimmed))
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var e composePSEntry
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				return nil, fmt.Errorf("parse compose ps line: %w", err)
			}
			entries = append(entries, e)
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("scan compose ps output: %w", err)
		}
	}

	out := make([]ComposeService, 0, len(entries))
	for _, e := range entries {
		name := e.Service
		if name == "" {
			name = e.Name
		}
		health := e.Health
		if health == "" {
			health = "none"
		}
		out = append(out, ComposeService{
			Name:   name,
			State:  e.State,
			Health: health,
			Image:  e.Image,
		})
	}
	return out, nil
}

// composeProject is one entry of `docker compose ls --format json` (an array).
type composeProject struct {
	Name string `json:"Name"`
}

// composeProjectListed reports whether the given project name appears in the
// output of `docker compose ls --format json`.
func composeProjectListed(raw []byte, project string) bool {
	var projects []composeProject
	if err := json.Unmarshal(bytes.TrimSpace(raw), &projects); err != nil {
		return false
	}
	for _, p := range projects {
		if p.Name == project {
			return true
		}
	}
	return false
}
