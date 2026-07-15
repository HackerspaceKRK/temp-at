package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
)

// streamSetupTimeout bounds the connect/DESCRIBE/SETUP/PLAY handshake. A printer
// that reboots or drops its network mid-handshake would otherwise wedge the
// (non-ctx-aware) RTSP calls indefinitely.
const streamSetupTimeout = 20 * time.Second

// streamDataTimeout aborts a connection that has gone silent after PLAY. A
// half-open TCP socket left behind by a printer reboot makes c.Wait() block
// forever; this watchdog forces it to return so the manager can reconnect.
const streamDataTimeout = 15 * time.Second

// bambuVideoSource streams the printer's chamber camera over RTSPS
// (rtsps://bblp:<access_code>@<host>:322/streaming/live/1). The credentials
// are the same as the MQTT ones.
type bambuVideoSource struct {
	cfg BambuPrinterConfig
}

func (b *bambuVideoSource) PrinterID() string { return b.cfg.ID }

func (b *bambuVideoSource) OpenStream(ctx context.Context, sink VideoSink) error {
	port := b.cfg.RtspPort
	if port == 0 {
		port = 322
	}
	u, err := base.ParseURL(fmt.Sprintf("rtsps://%s:%d/streaming/live/1", b.cfg.Host, port))
	if err != nil {
		return fmt.Errorf("parse RTSP URL: %w", err)
	}
	u.User = url.UserPassword(b.cfg.Username, b.cfg.Password)

	c := &gortsplib.Client{
		Scheme:       u.Scheme,
		Host:         u.Host,
		TLSConfig:    &tls.Config{InsecureSkipVerify: b.cfg.InsecureSkipVerify},
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer c.Close()

	// lastData tracks when the most recent access unit arrived (UnixNano);
	// playing flips true once PLAY succeeds. Together they let one watchdog cover
	// both phases: a bounded handshake and, afterwards, a silent-connection guard.
	var lastData atomic.Int64
	var playing atomic.Bool
	start := time.Now()

	// Abort the blocking RTSP calls / Wait() below when the manager cancels the
	// stream, the handshake stalls, or the connection goes silent after PLAY.
	// Without this an unreachable or rebooted printer parks the (non-ctx-aware)
	// Describe/Setup/Play or Wait() calls forever.
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				c.Close()
				return
			case <-watchDone:
				return
			case <-ticker.C:
				if !playing.Load() {
					if time.Since(start) > streamSetupTimeout {
						log.Printf("[stream] printer %s camera handshake timed out after %s", b.cfg.ID, streamSetupTimeout)
						c.Close()
						return
					}
					continue
				}
				if last := lastData.Load(); last != 0 && time.Since(time.Unix(0, last)) > streamDataTimeout {
					log.Printf("[stream] printer %s camera silent for %s, reconnecting", b.cfg.ID, streamDataTimeout)
					c.Close()
					return
				}
			}
		}
	}()

	desc, _, err := c.Describe(u)
	if err != nil {
		return fmt.Errorf("describe: %w", err)
	}

	var forma *format.H264
	medi := desc.FindFormat(&forma)
	if medi == nil {
		return fmt.Errorf("no H264 track found")
	}

	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		return fmt.Errorf("create RTP decoder: %w", err)
	}

	// Seed parameters from the SDP when present; Bambu also sends them in-band.
	sps, pps := forma.SPS, forma.PPS
	if sps != nil && pps != nil {
		sink.SetParams(sps, pps)
	}

	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("setup: %w", err)
	}

	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		pts, ok := c.PacketPTS(medi, pkt)
		if !ok {
			return
		}
		au, err := rtpDec.Decode(pkt)
		if err != nil {
			return // fragmented/incomplete packets are expected
		}

		// Track in-band parameter set updates.
		changed := false
		for _, nalu := range au {
			if len(nalu) == 0 {
				continue
			}
			switch h264.NALUType(nalu[0] & 0x1F) {
			case h264.NALUTypeSPS:
				sps = nalu
				changed = true
			case h264.NALUTypePPS:
				pps = nalu
				changed = true
			}
		}
		if changed && sps != nil && pps != nil {
			sink.SetParams(sps, pps)
		}

		lastData.Store(time.Now().UnixNano())
		sink.WriteAccessUnit(au, pts)
	})

	if _, err := c.Play(nil); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	// Enter the streaming phase: the watchdog now guards against silence rather
	// than a stalled handshake. Seed lastData so the timeout is measured from now.
	lastData.Store(time.Now().UnixNano())
	playing.Store(true)
	log.Printf("[stream] printer %s camera playing (H264, SDP params: sps=%dB pps=%dB)",
		b.cfg.ID, len(forma.SPS), len(forma.PPS))

	err = c.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
