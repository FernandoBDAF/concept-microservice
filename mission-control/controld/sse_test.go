package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestBrokerFlushesDroppedCountOnClose covers the slow-client seam: when the
// broker closes while a subscriber still has an unflushed dropped count, that
// count must survive as pendingDrop so the SSE loop can emit the
// "dropped N lines" marker before the end event (not vanish silently).
func TestBrokerFlushesDroppedCountOnClose(t *testing.T) {
	b := newBroker(ringMax)
	_, sub, done, _ := b.subscribe()
	if done {
		t.Fatal("fresh broker reported done")
	}

	// Publish past the subscriber buffer without draining — a slow client.
	total := subBuffer + 44
	for i := 0; i < total; i++ {
		b.publish(fmt.Sprintf("line-%d", i))
	}
	b.close(endPayload("succeeded", 0))

	// Drain the buffered lines; the channel must be closed at the end.
	received := 0
	for range sub.ch {
		received++
	}
	if received != subBuffer {
		t.Fatalf("received %d buffered lines, want %d", received, subBuffer)
	}
	if sub.pendingDrop != total-subBuffer {
		t.Errorf("pendingDrop = %d, want %d", sub.pendingDrop, total-subBuffer)
	}
	// The marker the SSE loop renders from it names the count.
	marker := fmt.Sprintf(dropMarkerFmt, sub.pendingDrop)
	if !strings.Contains(marker, "44 line(s)") {
		t.Errorf("marker = %q", marker)
	}
}

// TestBrokerDropMarkerMidStream covers the pre-existing path: a client that
// drains after dropping gets the marker in-band once the channel has room.
func TestBrokerDropMarkerMidStream(t *testing.T) {
	b := newBroker(ringMax)
	_, sub, _, _ := b.subscribe()

	for i := 0; i < subBuffer+3; i++ {
		b.publish("x")
	}
	// Drain two slots, then publish again: the drop marker is flushed first
	// (one slot), the new line lands in the second.
	<-sub.ch
	<-sub.ch
	b.publish("after")
	drained := make([]string, 0, subBuffer+2)
	b.close(endPayload("succeeded", 0))
	for l := range sub.ch {
		drained = append(drained, l)
	}
	foundMarker := false
	for _, l := range drained {
		if strings.Contains(l, "dropped 3 line(s)") {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Errorf("no in-band drop marker; tail = %v", drained[len(drained)-3:])
	}
	if sub.pendingDrop != 0 {
		t.Errorf("pendingDrop = %d, want 0 (marker already flushed)", sub.pendingDrop)
	}
}
