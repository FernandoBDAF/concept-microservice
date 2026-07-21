"use client";

// The action modal is the teaching surface and the control plane in one:
// it shows the request, the RESOLVED make command, live streamed output, and
// the terminal state. It drives three entry points — launch (POST), a
// destructive-confirm gate, and reopening a known record (streaming only if
// that record is still running).

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, getJSON, postJSON, streamUrl } from "../lib/api";
import { actionStateClass, previewCommand, reportSummary } from "../lib/format";
import type {
  ActionRecord,
  ActionRequest,
  ActionState,
  ExperimentReport,
} from "../lib/types";
import { useCockpit, type ActionModalMode } from "../lib/store";

const MAX_LINES = 2000;
// Stream-recovery tuning: a dropped SSE connection retries with doubling
// backoff; once retries are spent we fall back to polling the record until
// the terminal state is known (or the modal closes).
const RECONNECT_MAX = 4;
const RECONNECT_BASE_MS = 1000;
const RECOVER_POLL_MS = 3000;

type Phase = "confirm" | "posting" | "streaming" | "done" | "error";

export function ActionModal({
  mode,
  onClose,
}: {
  mode: ActionModalMode;
  onClose: () => void;
}) {
  const { addToast, openRunning, bumpRuns } = useCockpit();

  const initialPhase: Phase =
    mode.kind === "launch"
      ? mode.confirm
        ? "confirm"
        : "posting"
      : mode.record.state === "running"
      ? "streaming"
      : "done";

  const [phase, setPhase] = useState<Phase>(initialPhase);
  const [actionId, setActionId] = useState<string | null>(
    mode.kind === "open" ? mode.record.id : null
  );
  const [command, setCommand] = useState<string>(
    mode.kind === "open"
      ? mode.record.command
      : previewCommand(
          mode.system,
          mode.request.target,
          mode.request.verb,
          mode.request.params
        ) ?? ""
  );
  const [resolved, setResolved] = useState<boolean>(mode.kind === "open");
  const [lines, setLines] = useState<string[]>([]);
  const [finalState, setFinalState] = useState<ActionState | null>(
    mode.kind === "open" && mode.record.state !== "running"
      ? mode.record.state
      : null
  );
  const [exitCode, setExitCode] = useState<number | undefined>(
    mode.kind === "open" ? mode.record.exit_code : undefined
  );
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [streamError, setStreamError] = useState<string | null>(null);
  // Scored-experiment report: present up-front when reopening a completed
  // record; for a fresh launch it is fetched once the run ends (the SSE end
  // event carries only state + exit_code).
  const [report, setReport] = useState<ExperimentReport | null>(
    mode.kind === "open" ? mode.record.report ?? null : null
  );

  const esRef = useRef<EventSource | null>(null);
  const outRef = useRef<HTMLPreElement | null>(null);
  // Stream-recovery bookkeeping (refs: the retry loop lives across renders).
  const endedRef = useRef(false);
  const reconnectAttemptsRef = useRef(0);
  const retryTimerRef = useRef<number | null>(null);
  const pollTimerRef = useRef<number | null>(null);
  const request = mode.kind === "launch" ? mode.request : mode.record.request;

  const appendLine = useCallback((line: string) => {
    setLines((prev) => {
      const next = prev.length >= MAX_LINES ? prev.slice(1) : prev.slice();
      next.push(line);
      return next;
    });
  }, []);

  // Best-effort: pull the just-completed record to surface its scored-run
  // report (assertion breakdown). The engine attaches the report before the
  // SSE end event fires, so a direct GET here is race-free. Absence is
  // normal — non-experiment verbs and older runs carry no report — so
  // failures stay silent.
  const fetchReport = useCallback(async (id: string) => {
    try {
      const rec = await getJSON<ActionRecord>(
        `/api/actions/${encodeURIComponent(id)}`
      );
      if (rec.report) setReport(rec.report);
    } catch {
      /* report is optional; leave it unset */
    }
  }, []);

  // Last-resort recovery once stream retries are spent: poll the record until
  // it reaches a terminal state, then surface state/exit code/report. Keeps
  // trying while controld is unreachable so the modal never sticks in an
  // unknown state when the daemon comes back.
  const pollForFinal = useCallback(
    function pollForFinal(id: string) {
      void (async () => {
        if (endedRef.current) return;
        try {
          const rec = await getJSON<ActionRecord>(
            `/api/actions/${encodeURIComponent(id)}`
          );
          if (rec.state !== "running") {
            endedRef.current = true;
            setFinalState(rec.state);
            setExitCode(rec.exit_code);
            if (rec.report) setReport(rec.report);
            setStreamError("stream lost — final state recovered from controld");
            setPhase("done");
            bumpRuns();
            return;
          }
        } catch {
          /* controld unreachable right now; keep trying below */
        }
        pollTimerRef.current = window.setTimeout(
          () => pollForFinal(id),
          RECOVER_POLL_MS
        );
      })();
    },
    [bumpRuns]
  );

  const startStream = useCallback(
    function startStream(id: string) {
      const es = new EventSource(streamUrl(id));
      esRef.current = es;
      es.onopen = () => {
        reconnectAttemptsRef.current = 0;
        setStreamError(null);
      };
      es.addEventListener("line", (e) => appendLine((e as MessageEvent).data));
      es.addEventListener("end", (e) => {
        endedRef.current = true;
        try {
          const payload = JSON.parse((e as MessageEvent).data) as {
            state: ActionState;
            exit_code?: number;
          };
          setFinalState(payload.state);
          setExitCode(payload.exit_code);
        } catch {
          setFinalState("succeeded");
        }
        setStreamError(null);
        setPhase("done");
        es.close();
        esRef.current = null;
        bumpRuns();
        void fetchReport(id);
      });
      es.onerror = () => {
        // Ignore errors after the end event or an intentional close.
        if (!esRef.current || endedRef.current) return;
        es.close();
        esRef.current = null;
        const attempt = ++reconnectAttemptsRef.current;
        if (attempt <= RECONNECT_MAX) {
          // Reconnecting is lossless: the broker replays its full output ring
          // on subscribe. Reset the buffer so replayed lines don't duplicate.
          setStreamError(
            `stream disconnected — reconnecting (${attempt}/${RECONNECT_MAX})…`
          );
          retryTimerRef.current = window.setTimeout(() => {
            retryTimerRef.current = null;
            setLines([]);
            startStream(id);
          }, RECONNECT_BASE_MS * 2 ** (attempt - 1));
        } else {
          setStreamError("stream lost — polling controld for the final state…");
          pollForFinal(id);
        }
      };
    },
    [appendLine, bumpRuns, fetchReport, pollForFinal]
  );

  const doPost = useCallback(async () => {
    if (mode.kind !== "launch") return;
    setPhase("posting");
    setErrorMsg(null);
    // The confirm gate stands in for the CLI's explicit confirm=true: once the
    // user clicks through it, carry that consent on the wire — controld
    // rejects destructive verbs without params.confirm="true".
    const body: ActionRequest = mode.confirm
      ? { ...mode.request, params: { ...mode.request.params, confirm: "true" } }
      : mode.request;
    try {
      const res = await postJSON<{ id: string; command: string }>(
        "/api/actions",
        body
      );
      setActionId(res.id);
      setCommand(res.command);
      setResolved(true);
      setPhase("streaming");
      bumpRuns();
      startStream(res.id);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        const runningId = err.runningId;
        addToast({
          tone: "warn",
          message: `An action is already running for ${mode.request.system}/${mode.request.target}.`,
          ...(runningId
            ? {
                action: {
                  label: "View running",
                  onClick: () => void openRunning(runningId),
                },
              }
            : {}),
        });
        onClose();
        return;
      }
      setErrorMsg(err instanceof Error ? err.message : String(err));
      setPhase("error");
    }
  }, [mode, addToast, openRunning, onClose, bumpRuns, startStream]);

  // Kick off on mount: auto-post (non-destructive launch) or stream (reopen of
  // a still-running record). Destructive launches wait on the confirm gate.
  useEffect(() => {
    if (mode.kind === "launch" && !mode.confirm) {
      void doPost();
    } else if (mode.kind === "open" && initialPhase === "streaming") {
      startStream(mode.record.id);
    }
    return () => {
      esRef.current?.close();
      esRef.current = null;
      if (retryTimerRef.current !== null)
        window.clearTimeout(retryTimerRef.current);
      if (pollTimerRef.current !== null)
        window.clearTimeout(pollTimerRef.current);
      endedRef.current = true; // stop any in-flight recovery poll
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Auto-scroll the output to the newest line.
  useEffect(() => {
    if (outRef.current) outRef.current.scrollTop = outRef.current.scrollHeight;
  }, [lines]);

  // Closing mid-stream detaches the view, not the action — it keeps running
  // on controld. Hint at that (and the re-attach path) before letting go.
  const requestClose = useCallback(() => {
    if (phase === "streaming") {
      const leave = window.confirm(
        "The action keeps running on controld after this view closes.\n" +
          'Re-attach any time: launch the same system/target again and pick "View running" — the full output replays.\n\nClose the view?'
      );
      if (!leave) return;
    }
    onClose();
  }, [phase, onClose]);

  // Escape closes (guarded while streaming).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") requestClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [requestClose]);

  const title = `${request.verb} · ${request.system} · ${request.target}`;
  const destructive = request.verb === "down";

  const paramEntries = Object.entries(request.params ?? {}).filter(
    ([, v]) => v !== undefined && v !== ""
  );

  return (
    <div className="overlay" onMouseDown={requestClose}>
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="modal-head">
          <span className="modal-title">
            <span className={destructive ? "verb-destructive" : "verb"}>
              {request.verb}
            </span>{" "}
            {request.system}
            <span className="prompt-sep"> :: </span>
            {request.target}
          </span>
          <button
            className="modal-close"
            aria-label="Close"
            onClick={requestClose}
          >
            ×
          </button>
        </div>

        <div className="modal-body">
          <div className="req-grid">
            <span className="req-key">system</span>
            <span>{request.system}</span>
            <span className="req-key">target</span>
            <span>{request.target}</span>
            <span className="req-key">verb</span>
            <span>{request.verb}</span>
            {paramEntries.map(([k, v]) => (
              <span key={k} style={{ display: "contents" }}>
                <span className="req-key">{k}</span>
                <span>{String(v)}</span>
              </span>
            ))}
          </div>

          <div className="cmd-line">
            <span className="cmd-arrow">this runs →</span>
            <code className={`cmd ${resolved ? "resolved" : "preview"}`}>
              {command || "(command resolved by controld)"}
            </code>
            {!resolved && command && (
              <span className="cmd-note">preview</span>
            )}
          </div>

          {report && (
            <div
              className={`report-panel ${report.passed ? "ok" : "bad"}`}
              role="group"
              aria-label="assertion results"
            >
              <div className="report-head">
                <span className={`badge ${report.passed ? "ok" : "bad"}`}>
                  {report.passed ? "PASS" : "FAIL"}
                </span>
                <span className="report-summary">{reportSummary(report)}</span>
              </div>
              {report.assertions.length > 0 && (
                <ul className="report-list">
                  {report.assertions.map((a, i) => (
                    <li
                      key={i}
                      className={`report-item ${a.passed ? "ok" : "bad"}`}
                    >
                      <span className="report-marker" aria-hidden="true">
                        {a.passed ? "✓" : "✗"}
                      </span>
                      <span className="report-name">{a.name}</span>
                      {a.detail && (
                        <pre className="report-detail">{a.detail}</pre>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}

          {phase === "confirm" && (
            <div className="confirm-box">
              <p>
                <strong>Destructive action.</strong> This will tear down{" "}
                <code>{request.system}</code> on <code>{request.target}</code>.
              </p>
              <div className="confirm-actions">
                <button className="btn-danger" onClick={() => void doPost()}>
                  Confirm — run it
                </button>
                <button className="btn-ghost" onClick={onClose}>
                  Cancel
                </button>
              </div>
            </div>
          )}

          {phase === "posting" && (
            <div className="notice">submitting action to controld…</div>
          )}

          {phase === "error" && errorMsg && (
            <div className="error-box">
              <strong>action rejected</strong>
              <span>{errorMsg}</span>
            </div>
          )}

          {(phase === "streaming" || phase === "done") && (
            <>
              <div className="output-label">
                output
                {actionId && <span className="output-id"> · {actionId}</span>}
              </div>
              <pre className="output" ref={outRef}>
                {lines.length === 0
                  ? finalState
                    ? "(no retained output for this completed action)"
                    : "waiting for output…"
                  : lines.join("\n")}
              </pre>
              {streamError && (
                <div className="stream-note">{streamError}</div>
              )}
            </>
          )}
        </div>

        <div className="modal-foot">
          {finalState ? (
            <span className={`badge ${actionStateClass(finalState)}`}>
              {finalState.toUpperCase()}
              {exitCode !== undefined ? ` · exit ${exitCode}` : ""}
            </span>
          ) : phase === "streaming" ? (
            <span className="badge warn">RUNNING</span>
          ) : (
            <span className="prompt-meta">idle</span>
          )}
          <button
            className="btn-ghost"
            style={{ marginLeft: "auto" }}
            onClick={requestClose}
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
