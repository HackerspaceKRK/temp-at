package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Anycubic "uart-wifi" protocol (Photon Mono X and friends), plain TCP on port
// 6000, no authentication. Requests are "<cmd>[,<args>...],end"; text responses
// echo the command name and are terminated by ",end", e.g.
//
//	-> sysinfo,end
//	<- sysinfo,Photon Mono X,V3.5.4,203535394134500A,MyWifi,end
//	-> getstatus,end
//	<- getstatus,print,<external>/<internal>.pwmx,<total layers>,<percent>,
//	   <layer>,<elapsed s>,<remaining s>,~37.12mL,UV,37.12,0.05,0,end
//	-> getfile,end
//	<- getfile,<external>/<internal>.pwmx,<external>/<internal>.pwmx,...,end
//	-> getPreview1,64.pwmx,end
//	<- getPreview1,64.pwmx,224,168,172,end            (width, height, ?)
//	-> getPreview2,64.pwmx,end
//	<- width*height raw little-endian RGB565 pixels, no header or terminator
//
// The printer accepts several commands on one connection. Previews cannot be
// fetched while a print is running. Text is GBK encoded
// (Chinese firmware). See https://github.com/adamoutler/anycubic-python.

const (
	anycubicDefaultPort     = 6000
	anycubicDialTimeout     = 5 * time.Second
	anycubicCommandTimeout  = 10 * time.Second
	anycubicPreviewTimeout  = 20 * time.Second
	anycubicMaxResponseSize = 1 << 20
	anycubicMaxPreviewDim   = 2048
)

type anycubicConn struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialAnycubic(host string, port int) (*anycubicConn, error) {
	if port == 0 {
		port = anycubicDefaultPort
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), anycubicDialTimeout)
	if err != nil {
		return nil, err
	}
	return &anycubicConn{conn: c, r: bufio.NewReader(c)}, nil
}

func (a *anycubicConn) Close() error { return a.conn.Close() }

// command sends cmd and returns the comma-separated response fields (trimmed,
// without the trailing "end"). fields[0] is the echoed command name.
func (a *anycubicConn) command(cmd string) ([]string, error) {
	name, _, _ := strings.Cut(cmd, ",")
	_ = a.conn.SetDeadline(time.Now().Add(anycubicCommandTimeout))
	if _, err := a.conn.Write([]byte(cmd)); err != nil {
		return nil, fmt.Errorf("sending %s: %w", name, err)
	}
	text, err := a.readTextResponse()
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", name, err)
	}
	fields := splitAnycubicFields(text)
	if !strings.EqualFold(fields[0], name) {
		return nil, fmt.Errorf("unexpected response to %s: %q", name, text)
	}
	return fields, nil
}

// readTextResponse reads up to and including the ",end" terminator and returns
// the decoded text without it.
func (a *anycubicConn) readTextResponse() (string, error) {
	var buf []byte
	for !bytes.HasSuffix(buf, []byte(",end")) {
		b, err := a.r.ReadByte()
		if err != nil {
			return "", err
		}
		buf = append(buf, b)
		if len(buf) > anycubicMaxResponseSize {
			return "", fmt.Errorf("response too large")
		}
	}
	return decodeAnycubicText(buf[:len(buf)-len(",end")]), nil
}

func decodeAnycubicText(b []byte) string {
	if s, err := simplifiedchinese.GBK.NewDecoder().Bytes(b); err == nil {
		return string(s)
	}
	return strings.ToValidUTF8(string(b), "�")
}

func splitAnycubicFields(text string) []string {
	fields := strings.Split(text, ",")
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	// Drop a trailing empty field (e.g. "getstatus,stop\r\n,end").
	for len(fields) > 1 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	return fields
}

type anycubicSysInfo struct {
	Model, Firmware, Serial, Wifi string
}

func (a *anycubicConn) sysInfo() (anycubicSysInfo, error) {
	f, err := a.command("sysinfo,end")
	if err != nil {
		return anycubicSysInfo{}, err
	}
	f = append(f, "", "", "", "") // tolerate short responses
	return anycubicSysInfo{Model: f[1], Firmware: f[2], Serial: f[3], Wifi: f[4]}, nil
}

type anycubicStatus struct {
	Status           string // "print", "pause", "stop", ...
	File             string // raw "<external>/<internal>" entry, as listed by getfile
	ExternalName     string // user-facing file name (may be truncated by the printer)
	InternalName     string // e.g. "64.pwmx", used for preview commands
	TotalLayers      int
	Progress         int // percent
	CurrentLayer     int
	ElapsedSeconds   int
	RemainingSeconds int
	ResinVolume      float64 // mL
	ExposureMode     string  // e.g. "UV"
	LayerHeight      float64 // mm
}

func (a *anycubicConn) status() (anycubicStatus, error) {
	f, err := a.command("getstatus,end")
	if err != nil {
		return anycubicStatus{}, err
	}
	return parseAnycubicStatus(f)
}

// anycubicStatusTail is the number of fixed fields after the file name.
const anycubicStatusTail = 10

func parseAnycubicStatus(f []string) (anycubicStatus, error) {
	if len(f) < 2 {
		return anycubicStatus{}, fmt.Errorf("short status response: %q", f)
	}
	st := anycubicStatus{Status: strings.ToLower(f[1])}
	if len(f) < 3 {
		return st, nil
	}
	// The file name field may itself contain commas, so parse the fixed-size
	// tail from the right and join whatever is left as the file name.
	fileEnd := len(f)
	if len(f) >= 3+anycubicStatusTail {
		fileEnd = len(f) - anycubicStatusTail
		tail := f[fileEnd:]
		st.TotalLayers = atoiLoose(tail[0])
		st.Progress = atoiLoose(tail[1])
		st.CurrentLayer = atoiLoose(tail[2])
		st.ElapsedSeconds = atoiLoose(tail[3])
		st.RemainingSeconds = atoiLoose(tail[4])
		st.ExposureMode = tail[6]
		st.ResinVolume, _ = strconv.ParseFloat(tail[7], 64)
		if st.ResinVolume == 0 { // fall back to the "~37.12mL" text field
			st.ResinVolume, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(tail[5], "~"), "mL"), 64)
		}
		st.LayerHeight, _ = strconv.ParseFloat(tail[8], 64)
	}
	file := strings.Join(f[2:fileEnd], ",")
	st.File = file
	st.InternalName = anycubicInternalName(file)
	st.ExternalName = file
	if i := strings.LastIndex(file, "/"); i >= 0 {
		st.ExternalName = file[:i]
	}
	return st, nil
}

func atoiLoose(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f)
	}
	return 0
}

// files lists the print files on the printer as raw "<external>/<internal>"
// entries (the same form as anycubicStatus.File).
func (a *anycubicConn) files() ([]string, error) {
	f, err := a.command("getfile,end")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range f[1:] {
		if strings.Contains(e, "/") {
			files = append(files, e)
		}
	}
	return files, nil
}

// anycubicInternalName extracts the internal name ("64.pwmx") from a raw
// "<external>/<internal>" file entry.
func anycubicInternalName(file string) string {
	return file[strings.LastIndex(file, "/")+1:]
}

// errAnycubicNoPreview is returned when the printer itself reports that it
// cannot provide a preview (as opposed to a transport failure).
var errAnycubicNoPreview = errors.New("printer has no preview")

// preview fetches the slicer preview of a file on the printer and returns it
// encoded as PNG.
func (a *anycubicConn) preview(internalName string) ([]byte, error) {
	f, err := a.command("getPreview1," + internalName + ",end")
	if err != nil {
		return nil, err
	}
	if len(f) < 4 {
		return nil, fmt.Errorf("%w: getPreview1 returned %q", errAnycubicNoPreview, strings.Join(f[1:], ","))
	}
	w, h := atoiLoose(f[2]), atoiLoose(f[3])
	if w <= 0 || h <= 0 || w > anycubicMaxPreviewDim || h > anycubicMaxPreviewDim {
		return nil, fmt.Errorf("%w: invalid preview size %dx%d", errAnycubicNoPreview, w, h)
	}

	_ = a.conn.SetDeadline(time.Now().Add(anycubicPreviewTimeout))
	if _, err := a.conn.Write([]byte("getPreview2," + internalName + ",end")); err != nil {
		return nil, fmt.Errorf("sending getPreview2: %w", err)
	}
	// On success the printer sends raw pixels only; detect a text error reply
	// instead of blocking until the deadline waiting for pixel data.
	const errPrefix = "getPreview2,"
	if head, err := a.r.Peek(len(errPrefix)); err == nil && strings.EqualFold(string(head), errPrefix) {
		text, _ := a.readTextResponse()
		return nil, fmt.Errorf("%w: getPreview2 returned %q", errAnycubicNoPreview, text)
	}
	raw := make([]byte, w*h*2)
	if _, err := io.ReadFull(a.r, raw); err != nil {
		return nil, fmt.Errorf("reading preview pixels: %w", err)
	}
	return encodeRGB565PNG(raw, w, h)
}

// encodeRGB565PNG converts little-endian RGB565 pixels to a PNG.
func encodeRGB565PNG(raw []byte, w, h int) ([]byte, error) {
	if len(raw) < w*h*2 {
		return nil, fmt.Errorf("short pixel data: %d < %d", len(raw), w*h*2)
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		v := uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
		r, g, b := uint8(v>>11&0x1f), uint8(v>>5&0x3f), uint8(v&0x1f)
		px := img.Pix[4*i : 4*i+4 : 4*i+4]
		px[0] = r<<3 | r>>2
		px[1] = g<<2 | g>>4
		px[2] = b<<3 | b>>2
		px[3] = 0xff
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
