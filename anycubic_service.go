package main

import (
	"errors"
	"fmt"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	anycubicPollInterval = 30 * time.Second
	// anycubicOfflineAfterFailures consecutive failed polls mark a printer offline.
	anycubicOfflineAfterFailures = 2
	// Previews cannot be fetched while printing, so every this many polls an
	// idle printer's file list is synced into PrinterThumbnailModel instead.
	anycubicPreviewSyncEvery = 5
	// anycubicPreviewSyncBudget bounds the time one sync spends downloading
	// previews (~2.5 s each) so the status poll stays timely; remaining files
	// are picked up by the next sync.
	anycubicPreviewSyncBudget = 20 * time.Second
	// anycubicFinishedRemainingSeconds: a print that disappears while at most
	// this much time was left is treated as finished rather than stopped. The
	// printer just reports "stop" either way, so it has to be inferred.
	anycubicFinishedRemainingSeconds = 90
)

// AnycubicPrinterState is the vdev state of an Anycubic resin printer: the
// shared PrinterBaseState plus what the uart-wifi protocol reports.
type AnycubicPrinterState struct {
	PrinterBaseState
	Model        string `json:"model"`
	Firmware     string `json:"firmware"`
	SerialNumber string `json:"serial_number"`
	WifiNetwork  string `json:"wifi_network"`
	// RawStatus is the printer's own status word (print, pause, stop, ...).
	RawStatus    string  `json:"raw_status"`
	ElapsedTime  int     `json:"elapsed_time"`  // minutes
	ResinVolume  float64 `json:"resin_volume"`  // mL needed for the whole print
	LayerHeight  float64 `json:"layer_height"`  // mm
	ExposureMode string  `json:"exposure_mode"` // e.g. "UV"
}

type anycubicPrinter struct {
	cfg AnycubicPrinterConfig

	mu sync.Mutex
	// cur is the state derived from the last successful poll (never "offline"),
	// used as the baseline for detecting print start/end across outages.
	cur    AnycubicPrinterState
	hasCur bool
	// published is what was last pushed to the vdev manager.
	published AnycubicPrinterState
	failures  int
	// Details of the active print from the last poll, for new-print and
	// finished-vs-stopped detection. fileKey is the raw "<external>/<internal>"
	// entry, also the PrinterThumbnailModel task name.
	fileKey      string
	elapsedSec   int
	remainingSec int
	pollCount    int
	// previewFailed holds file entries the printer refused a preview for, so
	// they are not retried (and logged) on every sync.
	previewFailed map[string]bool
}

// AnycubicService polls each configured Anycubic printer over TCP every 30 s
// and exposes a "printer" virtual device per printer.
type AnycubicService struct {
	vdev       *VdevManager
	push       *PushService
	db         *gorm.DB
	printers   map[string]*anycubicPrinter
	order      []*anycubicPrinter
	footerName string
}

func NewAnycubicService(cfg *Config, vdev *VdevManager, push *PushService, db *gorm.DB) *AnycubicService {
	s := &AnycubicService{
		vdev:       vdev,
		push:       push,
		db:         db,
		printers:   make(map[string]*anycubicPrinter),
		footerName: cfg.Branding.FooterName,
	}
	for _, pc := range cfg.AnycubicPrinters {
		if pc.ID == "" || pc.Host == "" {
			log.Printf("[anycubic] skipping printer with missing id/host: %q", pc.ID)
			continue
		}
		p := &anycubicPrinter{cfg: pc, previewFailed: make(map[string]bool)}
		p.published = offlineAnycubicState(AnycubicPrinterState{})
		s.printers[pc.ID] = p
		s.order = append(s.order, p)
		vdev.AddDevices([]*VirtualDevice{{
			ID:              pc.ID,
			Type:            VdevTypePrinter,
			State:           p.published,
			ProhibitControl: true,
		}})
	}
	return s
}

func (s *AnycubicService) Start() {
	for _, p := range s.order {
		go s.loop(p)
	}
}

func (s *AnycubicService) loop(p *anycubicPrinter) {
	ticker := time.NewTicker(anycubicPollInterval)
	defer ticker.Stop()
	for {
		s.poll(p, time.Now())
		<-ticker.C
	}
}

// poll opens one connection and fetches sysinfo + status; every
// anycubicPreviewSyncEvery polls of an idle printer it also syncs file previews.
func (s *AnycubicService) poll(p *anycubicPrinter, now time.Time) {
	conn, err := dialAnycubic(p.cfg.Host, p.cfg.Port)
	if err != nil {
		s.pollFailed(p, err)
		return
	}
	defer conn.Close()

	sys, err := conn.sysInfo()
	if err != nil {
		s.pollFailed(p, err)
		return
	}
	status, err := conn.status()
	if err != nil {
		s.pollFailed(p, err)
		return
	}

	p.mu.Lock()
	if p.failures >= anycubicOfflineAfterFailures {
		log.Printf("[anycubic] printer %s is back online", p.cfg.ID)
	}
	p.failures = 0
	prev, hadPrev := p.cur, p.hasCur
	cur := p.derive(sys, status, now)
	if cur.Active() {
		cur.HasThumbnail = s.hasThumbnail(p.cfg.ID, p.fileKey)
	}
	p.cur, p.hasCur = cur, true
	p.pollCount++
	syncPreviews := !cur.Active() && (p.pollCount-1)%anycubicPreviewSyncEvery == 0
	p.mu.Unlock()

	s.publish(p, cur)
	if hadPrev {
		sendPrintFinishedNotification(s.push, s.footerName, p.cfg.ID,
			prev.PrinterBaseState, cur.PrinterBaseState, anycubicFailDetail(cur))
	}

	if syncPreviews {
		s.syncPreviews(p, conn)
	}
}

// syncPreviews lists the files on the printer and downloads the previews not
// yet cached in PrinterThumbnailModel (task name = raw file entry, start time
// 0). Rows for files no longer on the printer are pruned.
func (s *AnycubicService) syncPreviews(p *anycubicPrinter, conn *anycubicConn) {
	files, err := conn.files()
	if err != nil {
		log.Printf("[anycubic] listing files on printer %s failed: %v", p.cfg.ID, err)
		return
	}
	if len(files) > 0 {
		s.db.Where("printer_id = ? AND gcode_start_time = 0 AND task_name NOT IN ?", p.cfg.ID, files).
			Delete(&PrinterThumbnailModel{})
	}
	var cached []string
	s.db.Model(&PrinterThumbnailModel{}).
		Where("printer_id = ? AND gcode_start_time = 0", p.cfg.ID).
		Pluck("task_name", &cached)
	have := make(map[string]bool, len(cached))
	for _, f := range cached {
		have[f] = true
	}

	deadline := time.Now().Add(anycubicPreviewSyncBudget)
	fetched := 0
	for _, file := range files {
		p.mu.Lock()
		failed := p.previewFailed[file]
		p.mu.Unlock()
		if have[file] || failed {
			continue
		}
		if time.Now().After(deadline) {
			break
		}
		png, err := conn.preview(anycubicInternalName(file))
		if errors.Is(err, errAnycubicNoPreview) {
			log.Printf("[anycubic] printer %s has no preview for %q: %v", p.cfg.ID, file, err)
			p.mu.Lock()
			p.previewFailed[file] = true
			p.mu.Unlock()
			continue
		}
		if err != nil {
			// Transport failure: the connection is unusable, retry next sync.
			log.Printf("[anycubic] fetching preview of %q from printer %s failed: %v", file, p.cfg.ID, err)
			break
		}
		row := PrinterThumbnailModel{PrinterID: p.cfg.ID, TaskName: file, PNG: png, CreatedAt: time.Now()}
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			log.Printf("[anycubic] storing preview of %q for printer %s failed: %v", file, p.cfg.ID, err)
			break
		}
		fetched++
	}
	if fetched > 0 {
		log.Printf("[anycubic] cached %d new preview(s) for printer %s", fetched, p.cfg.ID)
	}
}

func (s *AnycubicService) hasThumbnail(printerID, fileKey string) bool {
	if fileKey == "" {
		return false
	}
	var count int64
	s.db.Model(&PrinterThumbnailModel{}).
		Where("printer_id = ? AND task_name = ? AND gcode_start_time = 0", printerID, fileKey).
		Count(&count)
	return count > 0
}

// derive builds the new state from a poll result. Must hold p.mu.
func (p *anycubicPrinter) derive(sys anycubicSysInfo, status anycubicStatus, now time.Time) AnycubicPrinterState {
	prev := p.cur
	var st AnycubicPrinterState
	switch {
	case strings.HasPrefix(status.Status, "print"):
		st.State = "printing"
	case strings.HasPrefix(status.Status, "pause"):
		st.State = "paused"
	default:
		st.State = "idle"
	}

	switch {
	case st.Active():
		newPrint := !prev.Active() || p.fileKey != status.File ||
			status.ElapsedSeconds+60 < p.elapsedSec // same file restarted between polls
		if newPrint {
			st.StartedAt = now.Add(-time.Duration(status.ElapsedSeconds) * time.Second).UnixMilli()
			st.TaskID = fmt.Sprintf("%s@%d", status.InternalName, st.StartedAt/1000)
		} else {
			st.StartedAt, st.TaskID = prev.StartedAt, prev.TaskID
		}
		st.Filename = anycubicDisplayName(status.ExternalName)
		st.Progress = status.Progress
		st.LayerNum = status.CurrentLayer
		st.TotalLayerNum = status.TotalLayers
		st.RemainingTime = (status.RemainingSeconds + 59) / 60
		st.ElapsedTime = status.ElapsedSeconds / 60
		st.ResinVolume = status.ResinVolume
		st.LayerHeight = status.LayerHeight
		st.ExposureMode = status.ExposureMode
		p.fileKey, p.elapsedSec, p.remainingSec = status.File, status.ElapsedSeconds, status.RemainingSeconds

	case prev.Active():
		// The print just ended; the printer only says "stop", so infer whether
		// it completed from how close to the end it was at the last poll.
		st = prev
		st.State = "failed"
		finished := prev.Progress >= 99 || p.remainingSec <= anycubicFinishedRemainingSeconds ||
			(prev.TotalLayerNum > 0 && prev.LayerNum >= prev.TotalLayerNum)
		if finished {
			st.State = "finished"
			st.Progress = 100
			st.LayerNum = st.TotalLayerNum
		} else {
			st.PrintErrorText = fmt.Sprintf("Stopped at layer %d/%d (%d%%)", prev.LayerNum, prev.TotalLayerNum, prev.Progress)
		}
		st.RemainingTime = 0
		st.FinishedAt = now.UnixMilli()

	case prev.State == "finished" || prev.State == "failed":
		// Keep showing the last print's outcome until the next one starts.
		st = prev
	}

	st.Type = PrinterTypeAnycubic
	st.Online = true
	st.RawStatus = status.Status
	st.Model, st.Firmware, st.SerialNumber, st.WifiNetwork = sys.Model, sys.Firmware, sys.Serial, sys.Wifi
	return st
}

func anycubicFailDetail(st AnycubicPrinterState) string {
	if st.PrintErrorText == "" {
		return ""
	}
	return " — " + st.PrintErrorText
}

// anycubicDisplayName strips the slice-file extension (.pwmx, .pwmb, ...)
// from the external file name. Names may be truncated by the printer, so only
// known extensions are removed.
func anycubicDisplayName(name string) string {
	ext := path.Ext(name)
	if strings.HasPrefix(strings.ToLower(ext), ".pw") || ext == "." {
		return strings.TrimSuffix(name, ext)
	}
	return name
}

func offlineAnycubicState(st AnycubicPrinterState) AnycubicPrinterState {
	st.Type = PrinterTypeAnycubic
	st.State = "offline"
	st.Online = false
	return st
}

func (s *AnycubicService) pollFailed(p *anycubicPrinter, err error) {
	p.mu.Lock()
	p.failures++
	failures := p.failures
	goOffline := failures >= anycubicOfflineAfterFailures && p.published.Online
	st := offlineAnycubicState(p.cur)
	p.mu.Unlock()

	if failures == 1 {
		log.Printf("[anycubic] poll of printer %s failed: %v", p.cfg.ID, err)
	}
	if goOffline {
		log.Printf("[anycubic] printer %s is offline", p.cfg.ID)
		s.publish(p, st)
	}
}

// publish pushes the state into the vdev manager (which dedupes unchanged states).
func (s *AnycubicService) publish(p *anycubicPrinter, st AnycubicPrinterState) {
	p.mu.Lock()
	p.published = st
	p.mu.Unlock()
	s.vdev.ApplyUpdates([]*VirtualDeviceUpdate{{Name: p.cfg.ID, State: st}})
}

// CurrentTaskID returns the task id of the current/last print on the printer.
func (s *AnycubicService) CurrentTaskID(printerID string) string {
	p, ok := s.printers[printerID]
	if !ok {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cur.TaskID
}

// CurrentThumbnail returns the cached preview PNG of the current/last print.
func (s *AnycubicService) CurrentThumbnail(printerID string) ([]byte, error) {
	p, ok := s.printers[printerID]
	if !ok {
		return nil, fmt.Errorf("unknown printer %q", printerID)
	}
	p.mu.Lock()
	fileKey := p.fileKey
	hasPrint := p.cur.TaskID != ""
	p.mu.Unlock()
	if !hasPrint || fileKey == "" {
		return nil, fmt.Errorf("no print on printer %q", printerID)
	}
	var row PrinterThumbnailModel
	err := s.db.Where("printer_id = ? AND task_name = ? AND gcode_start_time = 0", printerID, fileKey).
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return row.PNG, nil
}
