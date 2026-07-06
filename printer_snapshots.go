package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// PrinterSnapshotSink receives freshly captured camera snapshots and knows
// whether a printer is worth polling. Implemented by BambuService (and by
// future printer services, e.g. OctoPrint).
type PrinterSnapshotSink interface {
	Online(printerID string) bool
	SetSnapshot(printerID string, images []SnapshotImage, lowResPreview string)
}

// PrinterSnapshotter captures a camera frame from every streaming-capable
// printer once a minute. It taps the shared StreamManager, so a running live
// stream is reused and the printer never sees a second connection; when idle,
// the manager's linger keeps connection churn low.
//
// H.264 -> JPEG decoding is delegated to the ffmpeg binary (there is no pure-Go
// H.264 decoder); without ffmpeg installed, snapshots are disabled.
type PrinterSnapshotter struct {
	registry   *PrinterStreamRegistry
	store      *SnapshotStore
	sink       PrinterSnapshotSink
	ffmpegPath string
}

func NewPrinterSnapshotter(registry *PrinterStreamRegistry, store *SnapshotStore, sink PrinterSnapshotSink) *PrinterSnapshotter {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("[printer-snapshot] ffmpeg not found in PATH, printer camera snapshots disabled: %v", err)
		ffmpegPath = ""
	}
	return &PrinterSnapshotter{
		registry:   registry,
		store:      store,
		sink:       sink,
		ffmpegPath: ffmpegPath,
	}
}

func (ps *PrinterSnapshotter) Start() {
	if ps.ffmpegPath == "" {
		return
	}
	for _, id := range ps.registry.PrinterIDs() {
		go ps.loop(id)
	}
}

func (ps *PrinterSnapshotter) loop(printerID string) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		if ps.sink.Online(printerID) {
			if err := ps.capture(printerID); err != nil {
				log.Printf("[printer-snapshot] capture failed for printer %s: %v", printerID, err)
			}
		}
		<-ticker.C
	}
}

func (ps *PrinterSnapshotter) capture(printerID string) error {
	mgr, ok := ps.registry.Manager(printerID)
	if !ok {
		return fmt.Errorf("no stream manager")
	}

	// Waiting for a keyframe takes up to one GOP (~1-2 s) plus the connection
	// handshake when the stream isn't already running.
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sps, pps, au, err := mgr.SnapshotAU(ctx)
	if err != nil {
		return fmt.Errorf("waiting for keyframe: %w", err)
	}

	jpegBytes, err := ps.decodeToJPEG(ctx, sps, pps, au)
	if err != nil {
		return err
	}

	ts := time.Now().Unix()
	images, lowRes, err := ps.store.BuildVariants(jpegBytes, "printer_"+sanitizeSnapshotKey(printerID), ts)
	if err != nil {
		return fmt.Errorf("building variants: %w", err)
	}
	ps.sink.SetSnapshot(printerID, images, lowRes)
	log.Printf("[printer-snapshot] captured snapshot for printer %s (%d bytes, %d variant(s), took %s)",
		printerID, len(jpegBytes), len(images), time.Since(started).Round(time.Millisecond))
	return nil
}

// decodeToJPEG feeds an Annex-B encoded IDR access unit into ffmpeg and reads
// back a single JPEG frame.
func (ps *PrinterSnapshotter) decodeToJPEG(ctx context.Context, sps, pps []byte, au [][]byte) ([]byte, error) {
	startCode := []byte{0, 0, 0, 1}
	var annexb bytes.Buffer
	annexb.Write(startCode)
	annexb.Write(sps)
	annexb.Write(startCode)
	annexb.Write(pps)
	for _, nalu := range au {
		annexb.Write(startCode)
		annexb.Write(nalu)
	}

	cmd := exec.CommandContext(ctx, ps.ffmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-f", "h264", "-i", "-",
		"-frames:v", "1",
		"-c:v", "mjpeg", "-q:v", "3",
		"-f", "image2", "-",
	)
	cmd.Stdin = &annexb
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w (%s)", err, strings.TrimSpace(errBuf.String()))
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output (%s)", strings.TrimSpace(errBuf.String()))
	}
	return out.Bytes(), nil
}

// sanitizeSnapshotKey makes a printer id safe for use in a single URL path
// segment (ids may contain slashes).
func sanitizeSnapshotKey(id string) string {
	return strings.NewReplacer("/", "-", "?", "-", "#", "-", " ", "-").Replace(id)
}
