package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
)

// Wire protocol for /api/v1/printer-stream: binary WebSocket messages with a
// 1-byte type prefix.
const (
	streamMsgStatus byte = 0x00 // JSON: {"codec": "...", "error": "..."}
	streamMsgInit   byte = 0x01 // fMP4 initialization segment (ftyp+moov)
	streamMsgMedia  byte = 0x02 // fMP4 media segment (moof+mdat)
)

// streamLingerDelay keeps the printer connection open briefly after the last
// subscriber leaves, so the minute-snapshotter and page reloads don't churn it.
const streamLingerDelay = 5 * time.Second

// streamPartMaxDuration caps how much video accumulates before a media
// segment is flushed to subscribers (90 kHz units; 45000 = 500 ms).
const streamPartMaxDuration = 45000

// StreamSubscriber receives framed protocol messages over C. The channel is
// closed by Unsubscribe; sends are non-blocking (slow consumers drop segments
// and resynchronize at the next keyframe).
type StreamSubscriber struct {
	C chan []byte
	// needKeyframe: don't deliver media segments until one starting with an
	// IDR frame arrives (fresh join, param change, or dropped segment).
	needKeyframe bool
}

type snapshotAUData struct {
	sps, pps []byte
	au       [][]byte
}

// StreamManager fans one printer camera connection out to any number of
// WebSocket viewers and the snapshotter. It opens the RTSP connection when the
// first subscriber appears and closes it (after a linger) when the last one
// leaves, guaranteeing at most one connection to the printer at any time.
type StreamManager struct {
	source PrinterVideoSource

	mu          sync.Mutex
	subs        map[*StreamSubscriber]struct{}
	snapWaiters []chan snapshotAUData
	alwaysOn    bool
	running     bool
	cancel      context.CancelFunc
	done        chan struct{} // closed when the current runLoop exits
	linger      *time.Timer
	lastErr     string
	// connected flips true on the first access unit of a connection and back to
	// false on disconnect, so a successful reconnect can clear a stale lastErr.
	connected bool

	// Codec parameters + cached fMP4 init segment.
	sps, pps []byte
	initSeg  []byte
	codecStr string

	// fMP4 muxing state. Samples are buffered until a part is flushed; the
	// last access unit is held back until the next one arrives so its
	// duration is known.
	seq          uint32
	basePTS      int64
	baseSet      bool
	lastSample   *fmp4.Sample
	lastPTS      int64
	partSamples  []*fmp4.Sample
	partStartDTS int64
	partDur      int64
	partIsIDR    bool
}

func NewStreamManager(source PrinterVideoSource) *StreamManager {
	return &StreamManager{
		source: source,
		subs:   make(map[*StreamSubscriber]struct{}),
	}
}

// Subscribe registers a new viewer and starts the camera connection if needed.
func (m *StreamManager) Subscribe() *StreamSubscriber {
	sub := &StreamSubscriber{C: make(chan []byte, 64), needKeyframe: true}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[sub] = struct{}{}
	log.Printf("[stream] subscriber joined printer %s (%d viewer(s), %d snapshot waiter(s))",
		m.source.PrinterID(), len(m.subs), len(m.snapWaiters))
	m.startLocked()
	if m.initSeg != nil {
		sub.C <- m.statusFrameLocked()
		sub.C <- append([]byte{streamMsgInit}, m.initSeg...)
	} else if m.lastErr != "" {
		sub.C <- m.statusFrameLocked()
	}
	return sub
}

// Unsubscribe removes a viewer; the last one out schedules the linger stop.
func (m *StreamManager) Unsubscribe(sub *StreamSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.subs[sub]; !ok {
		return
	}
	delete(m.subs, sub)
	close(sub.C)
	log.Printf("[stream] subscriber left printer %s (%d viewer(s), %d snapshot waiter(s))",
		m.source.PrinterID(), len(m.subs), len(m.snapWaiters))
	m.maybeLingerLocked()
}

// SnapshotAU starts the stream if needed and waits for the next keyframe,
// returning the codec parameters and the IDR access unit for JPEG decoding.
func (m *StreamManager) SnapshotAU(ctx context.Context) (sps, pps []byte, au [][]byte, err error) {
	ch := make(chan snapshotAUData, 1)
	m.mu.Lock()
	m.snapWaiters = append(m.snapWaiters, ch)
	m.startLocked()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		for i, w := range m.snapWaiters {
			if w == ch {
				m.snapWaiters = append(m.snapWaiters[:i], m.snapWaiters[i+1:]...)
				break
			}
		}
		m.maybeLingerLocked()
		m.mu.Unlock()
	}()

	select {
	case d := <-ch:
		return d.sps, d.pps, d.au, nil
	case <-ctx.Done():
		return nil, nil, nil, ctx.Err()
	}
}

// startLocked launches the connection loop if it isn't running. A new loop
// waits for the previous one to fully exit, so connections never overlap.
func (m *StreamManager) startLocked() {
	if m.linger != nil {
		m.linger.Stop()
		m.linger = nil
	}
	if m.running {
		return
	}
	m.running = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	prevDone := m.done
	done := make(chan struct{})
	m.done = done
	go func() {
		if prevDone != nil {
			<-prevDone
		}
		m.runLoop(ctx, done)
	}()
}

// maybeLingerLocked schedules the connection teardown once nobody is listening.
func (m *StreamManager) maybeLingerLocked() {
	if !m.running || m.alwaysOn || len(m.subs) > 0 || len(m.snapWaiters) > 0 || m.linger != nil {
		return
	}
	m.linger = time.AfterFunc(streamLingerDelay, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.linger = nil
		if m.running && len(m.subs) == 0 && len(m.snapWaiters) == 0 {
			m.cancel()
			m.running = false
		}
	})
}

// SetAlwaysOn controls whether the camera connection should stay up even when
// there are no viewers/snapshot waiters. When enabled, the stream loop runs
// continuously and reconnects on failures.
func (m *StreamManager) SetAlwaysOn(alwaysOn bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alwaysOn = alwaysOn
	if alwaysOn {
		m.startLocked()
		return
	}
	m.maybeLingerLocked()
}

// runLoop keeps the camera connection alive while subscribers exist.
func (m *StreamManager) runLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	for {
		log.Printf("[stream] connecting to camera of printer %s", m.source.PrinterID())
		err := m.source.OpenStream(ctx, m)
		if ctx.Err() != nil {
			log.Printf("[stream] disconnected from camera of printer %s", m.source.PrinterID())
			return
		}
		msg := "stream ended"
		if err != nil {
			msg = err.Error()
		}
		log.Printf("[stream] camera connection to printer %s failed: %s (retrying)", m.source.PrinterID(), msg)
		m.mu.Lock()
		m.lastErr = msg
		m.connected = false
		m.resetMuxLocked()
		// Re-arm existing viewers so the next connection re-delivers an init
		// segment and only resumes media on a keyframe against the fresh
		// timestamp base (avoids a decode freeze on reconnect).
		m.resyncSubsLocked()
		m.broadcastLocked(m.statusFrameLocked(), false)
		m.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// resetMuxLocked clears per-connection muxing state so a reconnect starts a
// clean timestamp base and buffers.
func (m *StreamManager) resetMuxLocked() {
	m.baseSet = false
	m.lastSample = nil
	m.partSamples = nil
	m.partDur = 0
}

// resyncSubsLocked re-arms every subscriber to wait for a keyframe and resends
// the cached init segment, so viewers reset their SourceBuffer and resume
// cleanly after a reconnect (the printer resends byte-identical SPS/PPS, so
// SetParams alone wouldn't fire).
func (m *StreamManager) resyncSubsLocked() {
	if m.initSeg == nil {
		return
	}
	initFrame := append([]byte{streamMsgInit}, m.initSeg...)
	for sub := range m.subs {
		sub.needKeyframe = true
		select {
		case sub.C <- initFrame:
		default:
		}
	}
}

// SetParams implements VideoSink: (re)build the init segment on SPS/PPS change.
func (m *StreamManager) SetParams(sps, pps []byte) {
	if len(sps) == 0 || len(pps) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if bytes.Equal(sps, m.sps) && bytes.Equal(pps, m.pps) {
		return
	}
	m.sps = append([]byte(nil), sps...)
	m.pps = append([]byte(nil), pps...)

	init := fmp4.Init{Tracks: []*fmp4.InitTrack{{
		ID:        1,
		TimeScale: 90000,
		Codec:     &fmp4.CodecH264{SPS: m.sps, PPS: m.pps},
	}}}
	var buf seekablebuffer.Buffer
	if err := init.Marshal(&buf); err != nil {
		log.Printf("[stream] failed to build init segment for printer %s: %v", m.source.PrinterID(), err)
		return
	}
	m.initSeg = buf.Bytes()
	if len(sps) >= 4 {
		m.codecStr = fmt.Sprintf("avc1.%02X%02X%02X", sps[1], sps[2], sps[3])
	}
	m.lastErr = ""
	log.Printf("[stream] printer %s video parameters ready (codec %s, init segment %d bytes)",
		m.source.PrinterID(), m.codecStr, len(m.initSeg))

	// Every subscriber must reset its SourceBuffer and wait for a keyframe.
	m.broadcastLocked(m.statusFrameLocked(), false)
	initFrame := append([]byte{streamMsgInit}, m.initSeg...)
	for sub := range m.subs {
		sub.needKeyframe = true
		select {
		case sub.C <- initFrame:
		default:
		}
	}
}

// WriteAccessUnit implements VideoSink: mux one access unit into the pending
// media segment, flushing on keyframes and every ~500 ms.
func (m *StreamManager) WriteAccessUnit(au [][]byte, pts int64) {
	isIDR := h264.IsRandomAccess(au)

	m.mu.Lock()
	defer m.mu.Unlock()

	// First access unit of a (re)connection: clear any stale error and let
	// viewers know the stream is live again.
	if !m.connected {
		m.connected = true
		if m.lastErr != "" {
			m.lastErr = ""
			m.broadcastLocked(m.statusFrameLocked(), false)
		}
	}

	if isIDR && len(m.snapWaiters) > 0 && m.sps != nil && m.pps != nil {
		// Copy the NALUs: the RTP decoder may reuse its buffers after we return.
		auCopy := make([][]byte, len(au))
		for i, nalu := range au {
			auCopy[i] = append([]byte(nil), nalu...)
		}
		d := snapshotAUData{sps: m.sps, pps: m.pps, au: auCopy}
		for _, ch := range m.snapWaiters {
			select {
			case ch <- d:
			default:
			}
		}
	}

	if m.initSeg == nil {
		return
	}

	// Parameter sets and access-unit delimiters don't belong in avc1 samples.
	filtered := au[:0:0]
	for _, nalu := range au {
		if len(nalu) == 0 {
			continue
		}
		switch h264.NALUType(nalu[0] & 0x1F) {
		case h264.NALUTypeSPS, h264.NALUTypePPS, h264.NALUTypeAccessUnitDelimiter:
			continue
		}
		filtered = append(filtered, nalu)
	}
	if len(filtered) == 0 {
		return
	}

	if !m.baseSet {
		m.basePTS = pts
		m.baseSet = true
	}
	rel := pts - m.basePTS
	// Keep timestamps monotonic even across camera PTS resets.
	if m.lastSample != nil && rel <= m.lastPTS {
		rel = m.lastPTS + 3000 // assume ~30 fps for the odd bad timestamp
		m.basePTS = pts - rel
	}

	// Finalize the held-back sample now that its duration is known.
	if m.lastSample != nil {
		m.lastSample.Duration = uint32(rel - m.lastPTS)
		if len(m.partSamples) == 0 {
			m.partStartDTS = m.lastPTS
			m.partIsIDR = !m.lastSample.IsNonSyncSample
		}
		m.partSamples = append(m.partSamples, m.lastSample)
		m.partDur += rel - m.lastPTS
	}

	// Flush before buffering a keyframe so parts always start on an IDR, and
	// periodically to bound latency.
	if len(m.partSamples) > 0 && (isIDR || m.partDur >= streamPartMaxDuration) {
		m.flushPartLocked()
	}

	var s fmp4.Sample
	if err := s.FillH264(0, filtered); err != nil {
		log.Printf("[stream] failed to fill sample for printer %s: %v", m.source.PrinterID(), err)
		return
	}
	m.lastSample = &s
	m.lastPTS = rel
}

// flushPartLocked marshals the buffered samples into a media segment and fans
// it out to the subscribers.
func (m *StreamManager) flushPartLocked() {
	part := fmp4.Part{
		SequenceNumber: m.seq,
		Tracks: []*fmp4.PartTrack{{
			ID:       1,
			BaseTime: uint64(m.partStartDTS),
			Samples:  m.partSamples,
		}},
	}
	m.seq++
	var buf seekablebuffer.Buffer
	if err := part.Marshal(&buf); err != nil {
		log.Printf("[stream] failed to marshal media segment for printer %s: %v", m.source.PrinterID(), err)
	} else {
		frame := append([]byte{streamMsgMedia}, buf.Bytes()...)
		for sub := range m.subs {
			if sub.needKeyframe {
				if !m.partIsIDR {
					continue
				}
				sub.needKeyframe = false
			}
			select {
			case sub.C <- frame:
			default:
				// Slow consumer: drop and resync at the next keyframe.
				sub.needKeyframe = true
			}
		}
	}
	m.partSamples = nil
	m.partDur = 0
}

// broadcastLocked sends a frame to all subscribers (non-blocking). When
// keyframeGated is true, subscribers waiting for a keyframe are skipped.
func (m *StreamManager) broadcastLocked(frame []byte, keyframeGated bool) {
	for sub := range m.subs {
		if keyframeGated && sub.needKeyframe {
			continue
		}
		select {
		case sub.C <- frame:
		default:
		}
	}
}

func (m *StreamManager) statusFrameLocked() []byte {
	payload, _ := json.Marshal(struct {
		Codec string `json:"codec,omitempty"`
		Error string `json:"error,omitempty"`
	}{Codec: m.codecStr, Error: m.lastErr})
	return append([]byte{streamMsgStatus}, payload...)
}
