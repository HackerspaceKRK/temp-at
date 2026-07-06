package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// fakeVideoSource lets tests drive the sink manually and counts concurrent
// connections to assert the single-connection guarantee.
type fakeVideoSource struct {
	active      atomic.Int32
	maxActive   atomic.Int32
	connects    atomic.Int32
	sinkCh      chan VideoSink // receives the sink of each new "connection"
	blockUntil  chan struct{}  // closed by test to end OpenStream (simulates error)
}

func newFakeVideoSource() *fakeVideoSource {
	return &fakeVideoSource{
		sinkCh:     make(chan VideoSink, 16),
		blockUntil: make(chan struct{}),
	}
}

func (f *fakeVideoSource) PrinterID() string { return "fake" }

func (f *fakeVideoSource) OpenStream(ctx context.Context, sink VideoSink) error {
	n := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		old := f.maxActive.Load()
		if n <= old || f.maxActive.CompareAndSwap(old, n) {
			break
		}
	}
	f.connects.Add(1)
	f.sinkCh <- sink
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.blockUntil:
		return nil
	}
}

// Minimal valid-enough SPS/PPS for muxing (values from a real H.264 stream).
var (
	testSPS = []byte{0x67, 0x64, 0x00, 0x2A, 0xAC, 0x2B, 0x40, 0x28, 0x02, 0xDD, 0x00, 0xF1, 0x22, 0x6A}
	testPPS = []byte{0x68, 0xEE, 0x38, 0x80}
)

// idrAU/nonIdrAU are tiny fake access units; only the NALU type matters.
var (
	idrAU    = [][]byte{{0x65, 0x88, 0x84, 0x00}}
	nonIdrAU = [][]byte{{0x41, 0x9A, 0x24, 0x6C}}
)

func waitForSink(t *testing.T, f *fakeVideoSource) VideoSink {
	t.Helper()
	select {
	case s := <-f.sinkCh:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("source was not opened")
		return nil
	}
}

func recvFrame(t *testing.T, sub *StreamSubscriber, wantType byte) []byte {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case frame, ok := <-sub.C:
			if !ok {
				t.Fatal("subscriber channel closed while waiting for frame")
			}
			if frame[0] == wantType {
				return frame
			}
			// Skip other frame types (e.g. status before init).
		case <-deadline:
			t.Fatalf("timed out waiting for frame type %#x", wantType)
		}
	}
}

func TestStreamManager_SingleConnectionManyViewers(t *testing.T) {
	src := newFakeVideoSource()
	m := NewStreamManager(src)

	sub1 := m.Subscribe()
	sink := waitForSink(t, src)
	sub2 := m.Subscribe()
	sub3 := m.Subscribe()

	sink.SetParams(testSPS, testPPS)
	// Feed a few AUs so parts get flushed (IDR flush happens on the *next* IDR).
	sink.WriteAccessUnit(idrAU, 0)
	sink.WriteAccessUnit(nonIdrAU, 3000)
	sink.WriteAccessUnit(idrAU, 6000)

	for _, sub := range []*StreamSubscriber{sub1, sub2, sub3} {
		recvFrame(t, sub, streamMsgInit)
		media := recvFrame(t, sub, streamMsgMedia)
		if len(media) < 8 {
			t.Errorf("media segment too small: %d bytes", len(media))
		}
	}

	if got := src.maxActive.Load(); got != 1 {
		t.Errorf("max concurrent connections = %d, want 1", got)
	}
	if got := src.connects.Load(); got != 1 {
		t.Errorf("connects = %d, want 1", got)
	}

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)
	m.Unsubscribe(sub3)
}

func TestStreamManager_LateJoinerWaitsForKeyframe(t *testing.T) {
	src := newFakeVideoSource()
	m := NewStreamManager(src)

	sub1 := m.Subscribe()
	sink := waitForSink(t, src)
	sink.SetParams(testSPS, testPPS)
	sink.WriteAccessUnit(idrAU, 0)
	sink.WriteAccessUnit(nonIdrAU, 3000)

	// Late joiner: must get init immediately, then no media until an
	// IDR-starting part.
	sub2 := m.Subscribe()
	recvFrame(t, sub2, streamMsgInit)

	// Flush a non-IDR part (500ms elapses -> time based flush).
	sink.WriteAccessUnit(nonIdrAU, 50000)
	sink.WriteAccessUnit(nonIdrAU, 53000)
	// Now a keyframe: flushes pending part (non-IDR start, skipped for sub2),
	// then the next flush starts with the IDR.
	sink.WriteAccessUnit(idrAU, 56000)
	sink.WriteAccessUnit(nonIdrAU, 59000)
	sink.WriteAccessUnit(idrAU, 102000)

	media := recvFrame(t, sub2, streamMsgMedia)
	if media[0] != streamMsgMedia {
		t.Fatalf("expected media frame, got %#x", media[0])
	}

	m.Unsubscribe(sub1)
	m.Unsubscribe(sub2)
}

func TestStreamManager_LingerStopsConnection(t *testing.T) {
	src := newFakeVideoSource()
	m := NewStreamManager(src)

	sub := m.Subscribe()
	waitForSink(t, src)
	m.Unsubscribe(sub)

	// Connection must survive the linger window...
	time.Sleep(streamLingerDelay / 2)
	if src.active.Load() != 1 {
		t.Fatal("connection dropped before linger expired")
	}
	// ...and be torn down after it.
	deadline := time.Now().Add(streamLingerDelay + 2*time.Second)
	for src.active.Load() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("connection not closed after linger delay")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// A resubscribe reopens exactly one connection.
	sub2 := m.Subscribe()
	waitForSink(t, src)
	if got := src.maxActive.Load(); got != 1 {
		t.Errorf("max concurrent connections = %d, want 1", got)
	}
	m.Unsubscribe(sub2)
}

func TestStreamManager_SnapshotAU(t *testing.T) {
	src := newFakeVideoSource()
	m := NewStreamManager(src)

	type result struct {
		sps, pps []byte
		au       [][]byte
		err      error
	}
	resCh := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		sps, pps, au, err := m.SnapshotAU(ctx)
		resCh <- result{sps, pps, au, err}
	}()

	sink := waitForSink(t, src)
	sink.SetParams(testSPS, testPPS)
	sink.WriteAccessUnit(nonIdrAU, 0) // must not satisfy the waiter
	sink.WriteAccessUnit(idrAU, 3000)

	r := <-resCh
	if r.err != nil {
		t.Fatalf("SnapshotAU error: %v", r.err)
	}
	if len(r.sps) == 0 || len(r.pps) == 0 || len(r.au) == 0 {
		t.Fatalf("SnapshotAU returned empty data: %+v", r)
	}
	if r.au[0][0]&0x1F != 5 {
		t.Errorf("snapshot AU is not an IDR frame")
	}
}
