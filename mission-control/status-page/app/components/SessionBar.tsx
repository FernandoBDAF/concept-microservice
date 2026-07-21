"use client";

// Practice-session recorder. Start opens a session; while active it shows the
// title + elapsed clock and lets you attach a note, end it, or pull the
// markdown summary (the era-1 write-up, tool-assisted).

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, getJSON, getText, patchJSON, postJSON } from "../lib/api";
import { fmtElapsed } from "../lib/format";
import type { Session } from "../lib/types";
import { useCockpit } from "../lib/store";

export function SessionBar() {
  const { addToast } = useCockpit();
  const [session, setSession] = useState<Session | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [noteOpen, setNoteOpen] = useState(false);
  const [note, setNote] = useState("");
  const [summary, setSummary] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const noteRef = useRef<HTMLTextAreaElement | null>(null);

  const loadCurrent = useCallback(async () => {
    try {
      const s = await getJSON<Session>("/api/sessions/current");
      setSession(s);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setSession(null);
      }
      // Other errors (controld down) leave the bar quiet; it retries on mount.
    }
  }, []);

  useEffect(() => {
    void loadCurrent();
  }, [loadCurrent]);

  useEffect(() => {
    if (!session) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [session]);

  const start = useCallback(async () => {
    const title = window.prompt("Session title");
    if (!title) return;
    try {
      await postJSON<{ id: string }>("/api/sessions", { title });
      await loadCurrent();
      addToast({ tone: "info", message: `Session started: ${title}` });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        // A session is already open (perhaps from another tab or the CLI) —
        // adopt it instead of leaving the bar stuck on "Start session".
        await loadCurrent();
        addToast({
          tone: "info",
          message: "A session is already open — showing it.",
        });
        return;
      }
      addToast({
        tone: "error",
        message: err instanceof Error ? err.message : "Failed to start session",
      });
    }
  }, [addToast, loadCurrent]);

  const saveNote = useCallback(async () => {
    if (!session || !note.trim()) {
      setNoteOpen(false);
      return;
    }
    try {
      await patchJSON(`/api/sessions/${session.id}`, { note: note.trim() });
      addToast({ tone: "info", message: "Note added to session" });
      setNote("");
      setNoteOpen(false);
    } catch (err) {
      addToast({
        tone: "error",
        message: err instanceof Error ? err.message : "Failed to add note",
      });
    }
  }, [session, note, addToast]);

  const end = useCallback(async () => {
    if (!session) return;
    if (!window.confirm(`End session "${session.title}"?`)) return;
    try {
      await patchJSON(`/api/sessions/${session.id}`, { close: true });
      setSession(null);
      addToast({ tone: "info", message: "Session ended" });
    } catch (err) {
      addToast({
        tone: "error",
        message: err instanceof Error ? err.message : "Failed to end session",
      });
    }
  }, [session, addToast]);

  const openSummary = useCallback(async () => {
    if (!session) return;
    try {
      const md = await getText(`/api/sessions/${session.id}/summary`);
      setSummary(md);
      setCopied(false);
    } catch (err) {
      addToast({
        tone: "error",
        message: err instanceof Error ? err.message : "Failed to load summary",
      });
    }
  }, [session, addToast]);

  const copySummary = useCallback(async () => {
    if (summary == null) return;
    try {
      await navigator.clipboard.writeText(summary);
      setCopied(true);
    } catch {
      addToast({ tone: "warn", message: "Clipboard unavailable" });
    }
  }, [summary, addToast]);

  return (
    <div className="session-bar">
      {!session ? (
        <button className="btn-ghost" onClick={() => void start()}>
          ▶ Start session
        </button>
      ) : (
        <>
          <span className="session-dot" aria-hidden="true" />
          <span className="session-title" title={session.title}>
            {session.title}
          </span>
          <span className="session-clock">
            {fmtElapsed(session.started_at, now)}
          </span>
          <div className="session-actions">
            <button
              className="btn-ghost"
              onClick={() => {
                setNoteOpen((v) => !v);
                window.setTimeout(() => noteRef.current?.focus(), 0);
              }}
            >
              Add note
            </button>
            <button className="btn-ghost" onClick={() => void openSummary()}>
              Summary
            </button>
            <button className="btn-ghost" onClick={() => void end()}>
              End
            </button>
          </div>

          {noteOpen && (
            <div className="popover note-popover">
              <textarea
                ref={noteRef}
                className="note-input"
                rows={3}
                placeholder="Note to attach to this session…"
                value={note}
                onChange={(e) => setNote(e.target.value)}
              />
              <div className="popover-actions">
                <button className="btn-ghost" onClick={() => void saveNote()}>
                  Save note
                </button>
                <button className="btn-ghost" onClick={() => setNoteOpen(false)}>
                  Cancel
                </button>
              </div>
            </div>
          )}
        </>
      )}

      {summary != null && (
        <div className="overlay" onMouseDown={() => setSummary(null)}>
          <div
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label="Session summary"
            onMouseDown={(e) => e.stopPropagation()}
          >
            <div className="modal-head">
              <span className="modal-title">session summary</span>
              <button
                className="modal-close"
                aria-label="Close"
                onClick={() => setSummary(null)}
              >
                ×
              </button>
            </div>
            <div className="modal-body">
              <pre className="output summary-output">{summary}</pre>
            </div>
            <div className="modal-foot">
              <button className="btn-ghost" onClick={() => void copySummary()}>
                {copied ? "Copied ✓" : "Copy markdown"}
              </button>
              <button
                className="btn-ghost"
                style={{ marginLeft: "auto" }}
                onClick={() => setSummary(null)}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
