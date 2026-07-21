package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// junitFixture matches the exact shape scripts/experiments/run.py write_junit
// emits: a <testsuite> of <testcase>s, one per assertion, with a <failure>
// (message attr = note, text = "<label>: <note>") only on failing cases. Passing
// cases are self-closing (empty) testcases. ElementTree writes an XML decl with
// single-quoted attrs; encoding/xml parses it fine.
const junitFixture = `<?xml version='1.0' encoding='utf-8'?>
<testsuite name="exp-04" tests="3" failures="1" errors="0" time="12.345" timestamp="2026-07-19T12:00:00">
  <testcase classname="exp-04" name="promql sum(rabbitmq_queue_messages{queue=~&quot;.*-processing&quot;}) &lt;= 0" time="1.230" />
  <testcase classname="exp-04" name="http http://localhost:8080/ready status 200" time="0.450" />
  <testcase classname="exp-04" name="promql sum(rabbitmq_queue_messages{queue=~&quot;.+.dlq&quot;}) == 0" time="30.000">
    <failure message="actual 5 (no samples → 0)">promql sum(rabbitmq_queue_messages{queue=~".+.dlq"}) == 0: actual 5 (no samples → 0)</failure>
  </testcase>
</testsuite>
`

// junitAllPass is a report where every assertion passed (no <failure> nodes).
const junitAllPass = `<?xml version='1.0' encoding='utf-8'?>
<testsuite name="exp-02" tests="2" failures="0" errors="0" time="3.100" timestamp="2026-07-19T12:00:00">
  <testcase classname="exp-02" name="promql q &lt;= 0" time="1.000" />
  <testcase classname="exp-02" name="http http://localhost:8080/ready status 200" time="0.500" />
</testsuite>
`

// junitStepsFailed is the synthetic single-case report run.py emits when a step
// fails fast before assertions are polled.
const junitStepsFailed = `<?xml version='1.0' encoding='utf-8'?>
<testsuite name="exp-06" tests="1" failures="1" errors="0" time="0.500" timestamp="2026-07-19T12:00:00">
  <testcase classname="exp-06" name="steps" time="0.500">
    <failure message="step 1 failed (exit 2): make sim-outage">steps: step 1 failed (exit 2): make sim-outage</failure>
  </testcase>
</testsuite>
`

func writeReport(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

func TestParseExperimentReportMixed(t *testing.T) {
	dir := writeReport(t, "exp-04-20260719-120000.xml", junitFixture)
	rep, err := parseExperimentReport(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rep.Total != 3 || rep.Failed != 1 || rep.Passed {
		t.Fatalf("summary = %+v, want total 3 / failed 1 / passed false", rep)
	}
	if len(rep.Assertions) != 3 {
		t.Fatalf("assertions = %d, want 3", len(rep.Assertions))
	}
	if !rep.Assertions[0].Passed || rep.Assertions[0].Detail != "" {
		t.Errorf("assertion[0] = %+v, want passed with no detail", rep.Assertions[0])
	}
	f := rep.Assertions[2]
	if f.Passed {
		t.Errorf("assertion[2] should have failed: %+v", f)
	}
	if f.Detail != "actual 5 (no samples → 0)" {
		t.Errorf("failure detail = %q, want the runner's note", f.Detail)
	}
	if f.Name == "" {
		t.Errorf("failing assertion lost its name")
	}
}

func TestParseExperimentReportAllPass(t *testing.T) {
	dir := writeReport(t, "exp-02-20260719-120000.xml", junitAllPass)
	rep, err := parseExperimentReport(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !rep.Passed || rep.Total != 2 || rep.Failed != 0 {
		t.Fatalf("summary = %+v, want passed / total 2 / failed 0", rep)
	}
}

func TestParseExperimentReportStepsFailed(t *testing.T) {
	dir := writeReport(t, "exp-06-20260719-120000.xml", junitStepsFailed)
	rep, err := parseExperimentReport(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rep.Passed || rep.Total != 1 || rep.Failed != 1 {
		t.Fatalf("summary = %+v", rep)
	}
	if rep.Assertions[0].Name != "steps" {
		t.Errorf("synthetic case name = %q, want steps", rep.Assertions[0].Name)
	}
}

// junitErrorSkipped exercises the junit children run.py does not emit today
// but junit tooling does: <error> and <skipped> must not parse as passed.
const junitErrorSkipped = `<?xml version='1.0' encoding='utf-8'?>
<testsuite name="exp-07" tests="3" failures="0" errors="1" skipped="1" time="4.000" timestamp="2026-07-19T12:00:00">
  <testcase classname="exp-07" name="http /ready 200" time="0.400" />
  <testcase classname="exp-07" name="promql q == 0" time="1.000">
    <error message="prometheus unreachable">promql q == 0: prometheus unreachable</error>
  </testcase>
  <testcase classname="exp-07" name="cli check" time="0.000">
    <skipped message="needs kind target" />
  </testcase>
</testsuite>
`

func TestParseExperimentReportErrorAndSkipped(t *testing.T) {
	dir := writeReport(t, "exp-07-20260719-120000.xml", junitErrorSkipped)
	rep, err := parseExperimentReport(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// error + skipped both count into Failed so passed-count math
	// (total - failed) stays consistent; the run is not passed.
	if rep.Passed || rep.Total != 3 || rep.Failed != 2 {
		t.Fatalf("summary = %+v, want total 3 / failed 2 / passed false", rep)
	}
	if !rep.Assertions[0].Passed || rep.Assertions[0].Skipped {
		t.Errorf("assertion[0] = %+v, want plain pass", rep.Assertions[0])
	}
	errCase := rep.Assertions[1]
	if errCase.Passed || errCase.Skipped || errCase.Detail != "prometheus unreachable" {
		t.Errorf("error case = %+v, want failed with the error message", errCase)
	}
	skipCase := rep.Assertions[2]
	if skipCase.Passed || !skipCase.Skipped || skipCase.Detail != "needs kind target" {
		t.Errorf("skipped case = %+v, want not-passed + skipped mark + message", skipCase)
	}
}

func TestParseExperimentReportLenient(t *testing.T) {
	// Empty dir (no xml) → error, not a partial report.
	if _, err := parseExperimentReport(t.TempDir()); err == nil {
		t.Error("empty dir: want error, got nil")
	}
	// Missing dir → error.
	if _, err := parseExperimentReport(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("missing dir: want error, got nil")
	}
	// Malformed XML → error.
	bad := writeReport(t, "exp-bad-1.xml", "<testsuite><testcase not closed")
	if _, err := parseExperimentReport(bad); err == nil {
		t.Error("malformed xml: want error, got nil")
	}
}

// TestExecuteAttachesExperimentReport is the end-to-end wiring test: an
// experiment action whose command writes junit XML into $EXPERIMENT_REPORT_DIR
// must surface the parsed report in BOTH the in-memory snapshot and the
// persisted JSONL record. It also confirms the env var is exported to the child.
func TestExecuteAttachesExperimentReport(t *testing.T) {
	e := execEngine(t, Config{}) // RepoRoot = tempdir; store at <tmp>/runs

	// No single quotes in this XML so it embeds cleanly in a single-quoted sh
	// literal. run.py's real output additionally carries an <?xml?> decl; this
	// exercises the same testcase/failure structure the parser consumes.
	const xml = `<testsuite name="exp-99" tests="2" failures="1"><testcase name="promql q &lt;= 0" /><testcase name="http /ready 200"><failure message="status 503 != 200">http /ready 200: status 503 != 200</failure></testcase></testsuite>`

	id := "exp99report0001"
	key := runKey("labstack", "compose")
	cmd := `printf '%s' '` + xml + `' > "$EXPERIMENT_REPORT_DIR/exp-99-20260719-120000.xml"`
	st := &actionState{
		rec: &ActionRecord{
			ID:        id,
			Request:   ActionRequest{System: "labstack", Target: "compose", Verb: "experiment"},
			Command:   cmd,
			State:     "running",
			StartedAt: time.Now().UTC(),
		},
		broker: newBroker(ringMax),
	}
	e.mu.Lock()
	e.records[id] = st
	e.running[key] = id
	e.mu.Unlock()

	e.execute(st, key) // runs synchronously

	rec := st.snapshot()
	if rec.State != "succeeded" {
		t.Fatalf("state = %q, want succeeded (cmd should exit 0)", rec.State)
	}
	if rec.Report == nil {
		t.Fatal("report not attached to snapshot")
	}
	if rec.Report.Total != 2 || rec.Report.Failed != 1 || rec.Report.Passed {
		t.Fatalf("report = %+v, want total 2 / failed 1 / passed false", rec.Report)
	}

	// The report must also be in the persisted JSONL.
	runs, err := e.store.Runs(10)
	if err != nil {
		t.Fatalf("store runs: %v", err)
	}
	var persisted *ExperimentReport
	for _, r := range runs {
		if r.ID == id {
			persisted = r.Report
		}
	}
	if persisted == nil {
		t.Fatal("report not present in persisted record")
	}
	if persisted.Total != 2 || persisted.Failed != 1 {
		t.Errorf("persisted report = %+v", persisted)
	}
}

// TestExecuteNonExperimentNoReport confirms non-experiment verbs neither create
// a report dir nor attach a report (env untouched).
func TestExecuteNonExperimentNoReport(t *testing.T) {
	e := execEngine(t, Config{})
	id := "noreport00000001"
	key := runKey("labstack", "compose")
	st := &actionState{
		rec: &ActionRecord{
			ID:        id,
			Request:   ActionRequest{System: "labstack", Target: "compose", Verb: "up"},
			Command:   "true",
			State:     "running",
			StartedAt: time.Now().UTC(),
		},
		broker: newBroker(ringMax),
	}
	e.mu.Lock()
	e.records[id] = st
	e.running[key] = id
	e.mu.Unlock()

	e.execute(st, key)

	if rec := st.snapshot(); rec.Report != nil {
		t.Errorf("non-experiment verb attached a report: %+v", rec.Report)
	}
	if _, err := os.Stat(reportDirFor(e.store, id)); !os.IsNotExist(err) {
		t.Errorf("report dir created for non-experiment verb (err=%v)", err)
	}
}

func TestReportDirFor(t *testing.T) {
	if d := reportDirFor(nil, "abc"); d != "" {
		t.Errorf("nil store → %q, want empty", d)
	}
	store := NewStore("runs", quietLogger())
	d := reportDirFor(store, "deadbeef")
	if !filepath.IsAbs(d) {
		t.Errorf("report dir %q is not absolute", d)
	}
	if filepath.Base(d) != "deadbeef" || filepath.Base(filepath.Dir(d)) != "reports" {
		t.Errorf("report dir layout = %q, want <abs>/runs/reports/deadbeef", d)
	}
}
