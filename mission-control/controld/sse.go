package main

// sse.go — the per-action output broker and Server-Sent-Events handler.
//
// Each running action owns a broker: a bounded ring buffer (last 2000 lines)
// plus per-subscriber fanout channels. A new SSE subscriber replays the ring,
// then receives live lines, then a terminal "end" event. Slow subscribers
// drop lines (a "…dropped N lines…" marker is injected) rather than block the
// exec loop.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	ringMax        = 2000 // last N lines retained for replay
	subBuffer      = 256  // per-subscriber channel depth before dropping
	sseHeartbeat   = 15 * time.Second
	dropMarkerFmt  = "…controld: dropped %d line(s) to a slow client…"
	timeoutMarker  = "…controld: action timed out after %s — process group killed…"
	startupErrMark = "…controld: failed to start command: %s…"
)

// broker fans one action's merged stdout/stderr out to SSE subscribers and
// retains a replay ring.
type broker struct {
	mu      sync.Mutex
	ring    []string
	maxRing int
	subs    map[*subscriber]struct{}
	done    bool
	endData string
}

type subscriber struct {
	ch      chan string
	dropped int
}

func newBroker(maxRing int) *broker {
	return &broker{maxRing: maxRing, subs: map[*subscriber]struct{}{}}
}

// publish appends a line to the ring and fans it out to live subscribers.
func (b *broker) publish(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return
	}
	b.ring = append(b.ring, line)
	if len(b.ring) > b.maxRing {
		b.ring = b.ring[len(b.ring)-b.maxRing:]
	}
	for s := range b.subs {
		b.trySend(s, line)
	}
}

// trySend delivers a line without blocking; on a full channel it counts a drop
// and flushes a drop marker once the channel drains.
func (b *broker) trySend(s *subscriber, line string) {
	if s.dropped > 0 {
		select {
		case s.ch <- fmt.Sprintf(dropMarkerFmt, s.dropped):
			s.dropped = 0
		default:
			s.dropped++
			return
		}
	}
	select {
	case s.ch <- line:
	default:
		s.dropped++
	}
}

// subscribe atomically snapshots the replay ring and registers a live
// subscriber. If the action already finished it returns done=true with the end
// payload and no subscriber.
func (b *broker) subscribe() (replay []string, s *subscriber, done bool, endData string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	replay = append([]string(nil), b.ring...)
	if b.done {
		return replay, nil, true, b.endData
	}
	s = &subscriber{ch: make(chan string, subBuffer)}
	b.subs[s] = struct{}{}
	return replay, s, false, ""
}

func (b *broker) unsubscribe(s *subscriber) {
	b.mu.Lock()
	delete(b.subs, s)
	b.mu.Unlock()
}

// close marks the action finished, records the end payload, and closes every
// subscriber channel so their SSE loops emit the terminal event.
func (b *broker) close(endData string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return
	}
	b.done = true
	b.endData = endData
	for s := range b.subs {
		close(s.ch)
	}
	b.subs = map[*subscriber]struct{}{}
}

func (b *broker) endInfo() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.endData
}

// endPayload is the JSON body of the terminal "end" SSE event.
func endPayload(state string, exitCode int) string {
	data, _ := json.Marshal(struct {
		State    string `json:"state"`
		ExitCode int    `json:"exit_code"`
	}{state, exitCode})
	return string(data)
}

// serveSSE streams a broker to one HTTP client as text/event-stream. It
// replays the ring, then live lines, emits heartbeats, and honors client
// disconnect. Requires an http.Flusher.
func serveSSE(w http.ResponseWriter, r *http.Request, b *broker) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	writeLine := func(text string) {
		fmt.Fprintf(w, "event: line\ndata: %s\n\n", text)
		flusher.Flush()
	}
	writeEnd := func(data string) {
		if data == "" {
			data = endPayload("failed", -1)
		}
		fmt.Fprintf(w, "event: end\ndata: %s\n\n", data)
		flusher.Flush()
	}

	replay, sub, done, endData := b.subscribe()
	for _, l := range replay {
		writeLine(l)
	}
	if done {
		writeEnd(endData)
		return
	}
	defer b.unsubscribe(sub)

	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-sub.ch:
			if !ok {
				writeEnd(b.endInfo())
				return
			}
			writeLine(line)
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
