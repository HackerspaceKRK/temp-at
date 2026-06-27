package main

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jlaffaye/ftp"
	"gorm.io/gorm"
)

// bambuDefaultFtpPort is the implicit-FTPS port Bambu printers listen on.
const bambuDefaultFtpPort = 990

// thumbnailRelationshipType marks the package relationship that points at the
// plate preview PNG inside a .gcode.3mf (an OPC/zip container).
const thumbnailRelationshipType = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail"

// bambuThumbnailFailRetry is how long to wait before re-attempting a fetch that
// failed, so a missing/unreachable file doesn't hammer the printer.
const bambuThumbnailFailRetry = 2 * time.Minute

// ftpReaderAt adapts an FTP connection into an io.ReaderAt by issuing ranged
// RETR (REST+RETR) reads. archive/zip only needs the end-of-central-directory
// record plus a few small entries, so this avoids downloading the (potentially
// hundreds-of-MB) .3mf in full.
//
// A single RETR transfer is kept open between calls: because zip reads are
// sequential and mostly contiguous, a read at (or just ahead of) the current
// stream position is served from the open transfer instead of restarting it. A
// backward seek or a large forward jump closes the transfer and starts a new one
// at the requested offset. Not safe for concurrent use (zip does not need it).
type ftpReaderAt struct {
	conn *ftp.ServerConn
	path string
	size int64

	resp *ftp.Response // currently open transfer, nil if none
	pos  int64         // absolute offset of resp's next byte
}

// ftpReaderMaxForwardSkip caps how far ahead of the open stream we discard bytes
// to satisfy a read before giving up and restarting the transfer instead.
const ftpReaderMaxForwardSkip = 1 << 20 // 1 MiB

func (r *ftpReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("ftpReaderAt: negative offset")
	}
	if off >= r.size {
		return 0, io.EOF
	}

	r.positionStream(off)
	if r.resp == nil {
		resp, err := r.conn.RetrFrom(r.path, uint64(off))
		if err != nil {
			return 0, err
		}
		r.resp = resp
		r.pos = off
	}

	n, err := io.ReadFull(r.resp, p)
	r.pos += int64(n)
	if err != nil {
		// The transfer is no longer aligned for a subsequent read; drop it so the
		// next call restarts from the right offset.
		r.closeResp()
		if (err == io.ErrUnexpectedEOF || err == io.EOF) && n > 0 {
			// A short read at the end of the file is expected; zip tolerates it
			// as long as we report EOF.
			return n, io.EOF
		}
		return n, err
	}
	return n, nil
}

// positionStream prepares the open transfer to deliver bytes starting at off,
// reusing it when off is at or just ahead of the current position. Leaves r.resp
// nil if a new transfer is needed (the caller then opens one).
func (r *ftpReaderAt) positionStream(off int64) {
	if r.resp == nil {
		return
	}
	switch {
	case off == r.pos:
		// Already aligned; stream directly.
	case off > r.pos && off-r.pos <= ftpReaderMaxForwardSkip:
		// Discard the small gap (e.g. a zip local-header's filename/extra field)
		// to keep streaming instead of reopening the transfer.
		if _, err := io.CopyN(io.Discard, r.resp, off-r.pos); err != nil {
			r.closeResp()
			return
		}
		r.pos = off
	default:
		// Backward seek or large jump: cannot rewind a forward-only stream.
		r.closeResp()
	}
}

func (r *ftpReaderAt) closeResp() {
	if r.resp != nil {
		r.resp.Close()
		r.resp = nil
	}
}

// Close releases any open transfer. Safe to call multiple times.
func (r *ftpReaderAt) Close() error {
	r.closeResp()
	return nil
}

// opcRelationships mirrors the _rels/.rels XML in an OPC package.
type opcRelationships struct {
	XMLName       xml.Name `xml:"Relationships"`
	Relationships []struct {
		Target string `xml:"Target,attr"`
		Type   string `xml:"Type,attr"`
	} `xml:"Relationship"`
}

// fetchBambu3mfThumbnail connects to the printer over implicit FTPS, opens the
// given .3mf as a zip via ranged reads, resolves the thumbnail relationship in
// _rels/.rels and returns the referenced PNG bytes.
//
// candidates are remote paths tried in order (Bambu stores files in different
// locations depending on how the job was sent); the first that exists wins.
func fetchBambu3mfThumbnail(cfg BambuPrinterConfig, candidates []string) ([]byte, error) {
	port := cfg.FtpPort
	if port == 0 {
		port = bambuDefaultFtpPort
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	// Bambu's FTPS server requires the data connection to resume the control
	// connection's TLS session. A shared ClientSessionCache (plus a stable
	// ServerName so resumption keys match) makes Go offer the session ticket on
	// the data channel; without it the server drops the data transfer (0 bytes).
	tlsConf := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ClientSessionCache: tls.NewLRUClientSessionCache(8),
		ServerName:         cfg.Host,
	}

	conn, err := ftp.Dial(addr,
		ftp.DialWithTLS(tlsConf),
		ftp.DialWithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("ftp dial %s: %w", addr, err)
	}
	defer conn.Quit()

	if err := conn.Login(cfg.Username, cfg.Password); err != nil {
		return nil, fmt.Errorf("ftp login: %w", err)
	}

	var (
		path string
		size int64
	)
	for _, c := range candidates {
		if s, err := conn.FileSize(c); err == nil && s > 0 {
			path, size = c, s
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("3mf not found on printer (tried %v)", candidates)
	}

	ra := &ftpReaderAt{conn: conn, path: path, size: size}
	defer ra.Close()
	zr, err := zip.NewReader(ra, size)
	if err != nil {
		return nil, fmt.Errorf("open zip %s: %w", path, err)
	}

	target, err := bambuThumbnailTarget(zr)
	if err != nil {
		return nil, err
	}

	png, err := readZipFile(zr, target)
	if err != nil {
		return nil, fmt.Errorf("read thumbnail %q: %w", target, err)
	}
	return png, nil
}

// bambuThumbnailTarget parses _rels/.rels and returns the entry name of the
// thumbnail PNG (without the leading slash), e.g. "Metadata/plate_1.png".
func bambuThumbnailTarget(zr *zip.Reader) (string, error) {
	relsRaw, err := readZipFile(zr, "_rels/.rels")
	if err != nil {
		return "", fmt.Errorf("read _rels/.rels: %w", err)
	}
	var rels opcRelationships
	if err := xml.Unmarshal(relsRaw, &rels); err != nil {
		return "", fmt.Errorf("parse _rels/.rels: %w", err)
	}
	for _, rel := range rels.Relationships {
		if rel.Type == thumbnailRelationshipType {
			return strings.TrimPrefix(rel.Target, "/"), nil
		}
	}
	return "", fmt.Errorf("no thumbnail relationship in _rels/.rels")
}

// readZipFile reads a single entry from a zip reader by name. Zip entry names
// never start with a slash, so a leading slash from an OPC target is trimmed.
func readZipFile(zr *zip.Reader, name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("entry %q not found in zip", name)
}

// thumbAttempt tracks the in-memory state of a single (printer,task,start) key.
type thumbAttempt struct {
	cached     bool      // a row exists in the DB
	inflight   bool      // a fetch goroutine is currently running
	lastFailed time.Time // last failed attempt, for backoff
}

// bambuThumbnailCache fetches print-plate thumbnails over FTPS and persists them
// in SQLite, keyed by (printerID, taskName, gcodeStartTime), so a printer is hit
// at most once per print. An in-memory map guards against duplicate/looping
// fetches without a DB round-trip on every MQTT message.
type bambuThumbnailCache struct {
	db *gorm.DB

	// onCached is invoked (printerID) after a thumbnail is successfully stored,
	// so the owner can republish device state with HasThumbnail=true.
	onCached func(printerID string)

	mu    sync.Mutex
	state map[string]*thumbAttempt
}

func newBambuThumbnailCache(db *gorm.DB, onCached func(printerID string)) *bambuThumbnailCache {
	return &bambuThumbnailCache{
		db:       db,
		onCached: onCached,
		state:    make(map[string]*thumbAttempt),
	}
}

func thumbKey(printerID, taskName string, startMillis int64) string {
	return printerID + "|" + taskName + "|" + strconv.FormatInt(startMillis, 10)
}

// Has reports whether a thumbnail for the key is already cached. Once an
// in-memory attempt record exists (created here on first miss, or by Ensure),
// it is trusted so this is not a DB round-trip on every MQTT message; the DB is
// only consulted once per key (to pick up thumbnails cached in a prior run).
func (c *bambuThumbnailCache) Has(printerID, taskName string, startMillis int64) bool {
	if c == nil || taskName == "" {
		return false
	}
	key := thumbKey(printerID, taskName, startMillis)

	c.mu.Lock()
	if a := c.state[key]; a != nil {
		cached := a.cached
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	var count int64
	c.db.Model(&BambuThumbnailModel{}).
		Where("printer_id = ? AND task_name = ? AND gcode_start_time = ?", printerID, taskName, startMillis).
		Count(&count)
	if count > 0 {
		c.markCached(key)
		return true
	}
	// Record the miss so subsequent calls skip the DB; Ensure will fetch it.
	c.mu.Lock()
	if c.state[key] == nil {
		c.state[key] = &thumbAttempt{}
	}
	c.mu.Unlock()
	return false
}

// Get loads the cached PNG bytes for a key, or an error if none is stored.
func (c *bambuThumbnailCache) Get(printerID, taskName string, startMillis int64) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("no thumbnail cache")
	}
	var row BambuThumbnailModel
	err := c.db.
		Where("printer_id = ? AND task_name = ? AND gcode_start_time = ?", printerID, taskName, startMillis).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return row.PNG, nil
}

func (c *bambuThumbnailCache) markCached(key string) {
	c.mu.Lock()
	a := c.state[key]
	if a == nil {
		a = &thumbAttempt{}
		c.state[key] = a
	}
	a.cached = true
	a.inflight = false
	c.mu.Unlock()
}

// Ensure asynchronously fetches and caches the thumbnail for the given print if
// it is not already cached, in-flight, or in failure backoff.
func (c *bambuThumbnailCache) Ensure(cfg BambuPrinterConfig, taskName string, startMillis int64) {
	if c == nil || taskName == "" {
		return
	}
	key := thumbKey(cfg.ID, taskName, startMillis)

	c.mu.Lock()
	a := c.state[key]
	if a == nil {
		a = &thumbAttempt{}
		c.state[key] = a
	}
	if a.cached || a.inflight || (!a.lastFailed.IsZero() && time.Since(a.lastFailed) < bambuThumbnailFailRetry) {
		c.mu.Unlock()
		return
	}
	a.inflight = true
	c.mu.Unlock()

	go c.fetch(cfg, taskName, startMillis, key)
}

func (c *bambuThumbnailCache) fetch(cfg BambuPrinterConfig, taskName string, startMillis int64, key string) {
	// Another process/run may already have stored it.
	var count int64
	c.db.Model(&BambuThumbnailModel{}).
		Where("printer_id = ? AND task_name = ? AND gcode_start_time = ?", cfg.ID, taskName, startMillis).
		Count(&count)
	if count > 0 {
		c.markCached(key)
		if c.onCached != nil {
			c.onCached(cfg.ID)
		}
		return
	}

	png, err := fetchBambu3mfThumbnail(cfg, bambu3mfCandidates(taskName))
	if err != nil {
		log.Printf("[bambu] thumbnail fetch failed for printer %s task %q: %v", cfg.ID, taskName, err)
		c.mu.Lock()
		if a := c.state[key]; a != nil {
			a.inflight = false
			a.lastFailed = time.Now()
		}
		c.mu.Unlock()
		return
	}

	row := BambuThumbnailModel{
		PrinterID:      cfg.ID,
		TaskName:       taskName,
		GcodeStartTime: startMillis,
		PNG:            png,
		CreatedAt:      time.Now(),
	}
	if err := c.db.Create(&row).Error; err != nil {
		log.Printf("[bambu] failed to store thumbnail for printer %s task %q: %v", cfg.ID, taskName, err)
		c.mu.Lock()
		if a := c.state[key]; a != nil {
			a.inflight = false
			a.lastFailed = time.Now()
		}
		c.mu.Unlock()
		return
	}

	log.Printf("[bambu] cached thumbnail (%d bytes) for printer %s task %q", len(png), cfg.ID, taskName)
	c.markCached(key)
	if c.onCached != nil {
		c.onCached(cfg.ID)
	}
}

// bambu3mfCandidates returns the FTP paths to try for a print's project file,
// covering local-sent (root) and cached-cloud (/cache/) locations.
func bambu3mfCandidates(taskName string) []string {
	name := taskName + ".gcode.3mf"
	return []string{
		name,
		"/" + name,
		"/cache/" + name,
		"cache/" + name,
	}
}

// handleBambuThumbnail serves the cached plate-preview PNG for a printer's
// current print. Public (like /api/v1/room-states, which already exposes printer
// filename/state) so it works on the kiosk dashboard.
func handleBambuThumbnail(c *fiber.Ctx) error {
	if bambuService == nil {
		return fiber.ErrNotFound
	}
	// "+" is a greedy param so printer IDs containing slashes (e.g.
	// "bambu/cnc/printer") are captured whole.
	png, err := bambuService.CurrentThumbnail(c.Params("+"))
	if err != nil || len(png) == 0 {
		return fiber.ErrNotFound
	}
	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "no-cache")
	return c.Status(fiber.StatusOK).Send(png)
}

// pngMagic is the 8-byte PNG signature, used by tests to sanity-check output.
var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// isPNG reports whether b begins with the PNG signature.
func isPNG(b []byte) bool {
	return bytes.HasPrefix(b, pngMagic)
}
