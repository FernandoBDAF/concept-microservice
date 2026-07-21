package main

// report.go — parse the scored-runner's junit-ish XML into the ActionRecord
// (v6-HANDOFF §3). The runner (scripts/experiments/run.py, write_junit) emits
// one report per `make experiment` run to $EXPERIMENT_REPORT_DIR/<id>-<ts>.xml:
//
//   <?xml version='1.0' encoding='utf-8'?>
//   <testsuite name="exp-04" tests="2" failures="1" errors="0"
//              time="12.345" timestamp="2026-07-19T12:00:00">
//     <testcase classname="exp-04" name="promql ... <= 0" time="1.234" />
//     <testcase classname="exp-04" name="promql ... == 0" time="2.345">
//       <failure message="actual 5 (no samples → 0)">promql ... == 0: actual 5 ...</failure>
//     </testcase>
//   </testsuite>
//
// One <testcase> per assertion; a failing assertion carries a single <failure>
// whose message attr is the runner's note and whose text is "<label>: <note>".
// Passing assertions are empty (self-closing) testcases. A steps-failed run
// emits a single synthetic testcase named "steps".
//
// Parsing is deliberately lenient: a missing dir, absent XML, or malformed
// document yields (nil, error) — the caller logs a warn and leaves the record's
// Report nil, so the action's pass/fail stays exit-code driven.

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssertionResult is one scored assertion's outcome. (JSON contract — do not
// rename fields.)
type AssertionResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
	// Skipped marks a <skipped> testcase: not passed, but distinct from a
	// failure for display. It still counts into ExperimentReport.Failed so the
	// summary math (passed count = total - failed) stays consistent.
	Skipped bool `json:"skipped,omitempty"`
}

// ExperimentReport is the parsed summary of a scored run. (JSON contract — do
// not rename fields.)
type ExperimentReport struct {
	Passed     bool              `json:"passed"`
	Total      int               `json:"total"`
	Failed     int               `json:"failed"`
	Assertions []AssertionResult `json:"assertions"`
}

// junitSuite/junitCase/junitFailure mirror the subset of the junit-ish schema
// that run.py's write_junit emits.
type junitSuite struct {
	XMLName xml.Name    `xml:"testsuite"`
	Name    string      `xml:"name,attr"`
	Cases   []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string        `xml:"name,attr"`
	Failure *junitFailure `xml:"failure"`
	// Error and Skipped are not emitted by run.py today, but junit tooling
	// emits both — neither may silently parse as a passed assertion.
	Error   *junitFailure `xml:"error"`
	Skipped *junitFailure `xml:"skipped"`
}

// junitFailure is the shared shape of <failure>, <error>, and <skipped>
// children: a message attribute plus optional text content.
type junitFailure struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// reportDirFor returns the absolute per-action report dir under the store's
// run-history tree (reports/<action-id>/), or "" when there is no store. The
// path is absolutized because the child process execs from RepoRoot, not the
// controld working dir, so EXPERIMENT_REPORT_DIR must be an absolute path.
func reportDirFor(store *Store, id string) string {
	if store == nil {
		return ""
	}
	base := store.Dir()
	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}
	return filepath.Join(base, "reports", id)
}

// parseExperimentReport reads the newest *.xml in dir and maps it to an
// ExperimentReport. It returns an error (never a partial report) when the dir
// holds no XML or the document does not parse.
func parseExperimentReport(dir string) (*ExperimentReport, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return nil, fmt.Errorf("glob report dir %s: %w", dir, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no report xml in %s", dir)
	}
	path := newestReport(matches)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read report %s: %w", path, err)
	}
	var suite junitSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse report %s: %w", path, err)
	}

	rep := &ExperimentReport{
		Total:      len(suite.Cases),
		Assertions: make([]AssertionResult, 0, len(suite.Cases)),
	}
	for _, tc := range suite.Cases {
		ar := AssertionResult{Name: tc.Name, Passed: true}
		switch {
		case tc.Failure != nil:
			ar.Passed = false
			rep.Failed++
			ar.Detail = failureDetail(tc.Failure, "")
		case tc.Error != nil:
			// junit <error> (crashed, not asserted-false) → a failure with detail.
			ar.Passed = false
			rep.Failed++
			ar.Detail = failureDetail(tc.Error, "error")
		case tc.Skipped != nil:
			// junit <skipped> → not passed; marked distinctly, but counted into
			// Failed so passed-count math (total - failed) stays consistent.
			ar.Passed = false
			ar.Skipped = true
			rep.Failed++
			ar.Detail = failureDetail(tc.Skipped, "skipped")
		}
		rep.Assertions = append(rep.Assertions, ar)
	}
	rep.Passed = rep.Failed == 0
	return rep, nil
}

// failureDetail extracts the detail string from a <failure>/<error>/<skipped>
// child: message attr first, text content second, fallback last.
func failureDetail(f *junitFailure, fallback string) string {
	if d := strings.TrimSpace(f.Message); d != "" {
		return d
	}
	if d := strings.TrimSpace(f.Text); d != "" {
		return d
	}
	return fallback
}

// newestReport returns the path with the most recent mod time (falling back to
// the lexically-greatest path — timestamps in the filename sort chronologically
// — when stat fails). A fresh per-action dir normally holds exactly one XML.
func newestReport(paths []string) string {
	best := paths[0]
	var bestMod int64 = -1
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			if p > best {
				best = p
			}
			continue
		}
		if m := fi.ModTime().UnixNano(); m >= bestMod {
			bestMod, best = m, p
		}
	}
	return best
}
