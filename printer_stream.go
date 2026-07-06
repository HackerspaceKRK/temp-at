package main

import (
	"context"
	"sync"
)

// VideoSink receives demuxed H.264 video from a printer's camera.
// Implemented by StreamManager, which muxes it into fMP4 for browsers.
type VideoSink interface {
	// SetParams provides the current SPS/PPS; may be called again when the
	// encoder parameters change mid-stream.
	SetParams(sps, pps []byte)
	// WriteAccessUnit delivers the NALUs of one access unit. pts is in
	// 90 kHz clock units.
	WriteAccessUnit(au [][]byte, pts int64)
}

// PrinterVideoSource is a vendor-specific camera connection (Bambu today,
// OctoPrint etc. later). One OpenStream call equals one connection to the
// printer; the StreamManager guarantees at most one call is active at a time.
type PrinterVideoSource interface {
	PrinterID() string
	// OpenStream connects to the camera and pushes video into sink until ctx
	// is cancelled or a fatal error occurs. Blocking.
	OpenStream(ctx context.Context, sink VideoSink) error
}

// PrinterStreamRegistry holds one StreamManager per printer that has a camera.
type PrinterStreamRegistry struct {
	mu       sync.Mutex
	managers map[string]*StreamManager
}

func NewPrinterStreamRegistry() *PrinterStreamRegistry {
	return &PrinterStreamRegistry{managers: make(map[string]*StreamManager)}
}

func (r *PrinterStreamRegistry) Register(src PrinterVideoSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.managers[src.PrinterID()] = NewStreamManager(src)
}

func (r *PrinterStreamRegistry) Manager(printerID string) (*StreamManager, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.managers[printerID]
	return m, ok
}

// PrinterIDs returns the ids of all printers with a registered stream.
func (r *PrinterStreamRegistry) PrinterIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.managers))
	for id := range r.managers {
		ids = append(ids, id)
	}
	return ids
}
