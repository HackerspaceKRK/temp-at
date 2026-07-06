package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
)

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
		Scheme:      u.Scheme,
		Host:        u.Host,
		TLSConfig:   &tls.Config{InsecureSkipVerify: b.cfg.InsecureSkipVerify},
		ReadTimeout: 10 * time.Second,
	}
	if err := c.Start(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer c.Close()

	// Abort the blocking Wait() below when the manager cancels the stream.
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-watchDone:
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

		sink.WriteAccessUnit(au, pts)
	})

	if _, err := c.Play(nil); err != nil {
		return fmt.Errorf("play: %w", err)
	}
	log.Printf("[stream] printer %s camera playing (H264, SDP params: sps=%dB pps=%dB)",
		b.cfg.ID, len(forma.SPS), len(forma.PPS))

	err = c.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
