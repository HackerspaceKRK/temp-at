package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type BambuPrinterState struct {
	// State is one of: idle, printing, paused, finished, failed, offline.
	State         string  `json:"state"`
	Progress      int     `json:"progress"`       // percent, 0-100
	RemainingTime int     `json:"remaining_time"` // minutes
	Filename      string  `json:"filename"`
	ErrorCode     string  `json:"error_code"`
	TaskID        string  `json:"task_id"`
	LayerNum      int     `json:"layer_num"`
	TotalLayerNum int     `json:"total_layer_num"`
	// StartedAt is when the current/last print started (unix millis, 0 if unknown).
	StartedAt int64 `json:"started_at"`
	// FinishedAt is when the print finished or failed (unix millis, 0 while running).
	FinishedAt int64 `json:"finished_at"`
	NozzleTemp    float64 `json:"nozzle_temp"`
	NozzleTarget  float64 `json:"nozzle_target"`
	BedTemp       float64 `json:"bed_temp"`
	BedTarget     float64 `json:"bed_target"`
	ChamberTemp   float64 `json:"chamber_temp"`
	Online        bool    `json:"online"`
}

// bambuOfflineThreshold is how long without a message before a printer is
// considered offline (Bambu pushes roughly once per second while reachable).
const bambuOfflineThreshold = 60 * time.Second

type bambuPrinter struct {
	cfg    BambuPrinterConfig
	client mqtt.Client

	mu       sync.Mutex
	full     map[string]any // merged "print" object across incremental messages
	last     BambuPrinterState
	hasState bool
	lastMsg  time.Time
	// finishedAt is the locally observed time (unix millis) the current print
	// entered a finished/failed state; reset to 0 while not terminal.
	finishedAt int64
}

// BambuService maintains a TLS MQTT connection to each configured Bambu printer
// and exposes a derived "printer" virtual device per printer.
type BambuService struct {
	vdev     *VdevManager
	push     *PushService
	printers map[string]*bambuPrinter // keyed by printer ID
	order    []*bambuPrinter
	// footerName is the branding footer name, prefixed onto notification titles
	// so the user knows which space/installation a print belongs to.
	footerName string
}

func NewBambuService(cfg *Config, vdev *VdevManager, push *PushService) (*BambuService, error) {
	s := &BambuService{
		vdev:       vdev,
		push:       push,
		printers:   make(map[string]*bambuPrinter),
		footerName: cfg.Branding.FooterName,
	}

	for _, pc := range cfg.BambuPrinters {
		if pc.ID == "" || pc.SerialNumber == "" || pc.Host == "" {
			log.Printf("[bambu] skipping printer with missing id/serial/host: %+v", pc.ID)
			continue
		}
		p := &bambuPrinter{cfg: pc, full: make(map[string]any)}
		s.printers[pc.ID] = p
		s.order = append(s.order, p)

		// Register the vdev once; printer state is never persisted, so it always
		// starts in an unknown/offline state until the first message arrives.
		vdev.AddDevices([]*VirtualDevice{{
			ID:              pc.ID,
			Type:            VdevTypePrinter,
			State:           BambuPrinterState{State: "offline"},
			ProhibitControl: true,
		}})
	}

	return s, nil
}

// Start connects each printer's MQTT client and launches the offline watchdog.
func (s *BambuService) Start() {
	for _, p := range s.order {
		s.connectPrinter(p)
	}
	go s.watchdogLoop()
}

func (s *BambuService) connectPrinter(p *bambuPrinter) {
	port := p.cfg.Port
	if port == 0 {
		port = 8883
	}
	broker := fmt.Sprintf("ssl://%s:%d", p.cfg.Host, port)
	topic := fmt.Sprintf("device/%s/report", p.cfg.SerialNumber)

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(fmt.Sprintf("temp-at-bambu-%d", time.Now().UnixNano())).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetKeepAlive(30 * time.Second).
		SetConnectTimeout(8 * time.Second).
		SetOrderMatters(false).
		SetTLSConfig(&tls.Config{InsecureSkipVerify: p.cfg.InsecureSkipVerify})

	if p.cfg.Username != "" {
		opts.SetUsername(p.cfg.Username)
	}
	if p.cfg.Password != "" {
		opts.SetPassword(p.cfg.Password)
	}

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("[bambu] connected to %s (printer %s)", broker, p.cfg.ID)
		token := c.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
			s.handleMessage(p, msg.Payload())
		})
		if !token.WaitTimeout(5 * time.Second) {
			log.Printf("[bambu] subscription timeout for %s", topic)
		} else if err := token.Error(); err != nil {
			log.Printf("[bambu] failed to subscribe to %s: %v", topic, err)
		}
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Printf("[bambu] connection lost for printer %s: %v", p.cfg.ID, err)
	}

	p.client = mqtt.NewClient(opts)
	// Connect asynchronously; auto-reconnect keeps retrying so a printer that is
	// powered off at startup will still appear once it comes online.
	go func() {
		token := p.client.Connect()
		token.Wait()
		if err := token.Error(); err != nil {
			log.Printf("[bambu] initial connect failed for printer %s: %v", p.cfg.ID, err)
		}
	}()
}

// handleMessage merges an incoming report into the printer's full state and
// derives + publishes the small state.
func (s *BambuService) handleMessage(p *bambuPrinter, payload []byte) {
	var msg struct {
		Print map[string]any `json:"print"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Printf("[bambu] failed to parse report for printer %s: %v", p.cfg.ID, err)
		return
	}
	if msg.Print == nil {
		return // not a print status message (e.g. mc_print/info responses)
	}

	p.mu.Lock()
	for k, v := range msg.Print {
		p.full[k] = v
	}
	p.lastMsg = time.Now()
	newState := deriveBambuState(p.full)
	newState.Online = true
	prev := p.last

	// Record the finish/fail moment once, and clear it whenever a print is not
	// in a terminal state (e.g. a new print starts).
	if newState.State == "finished" || newState.State == "failed" {
		if p.finishedAt == 0 {
			p.finishedAt = time.Now().UnixMilli()
		}
	} else {
		p.finishedAt = 0
	}
	newState.FinishedAt = p.finishedAt

	hadState := p.hasState
	p.last = newState
	p.hasState = true
	p.mu.Unlock()

	s.publish(p, newState)

	if hadState {
		s.maybeNotify(p, prev, newState)
	}
}

// publish pushes the derived state into the vdev manager. VdevManager dedupes
// via value comparison, so repeated identical states do not spam the WebSocket.
func (s *BambuService) publish(p *bambuPrinter, st BambuPrinterState) {
	s.vdev.ApplyUpdates([]*VirtualDeviceUpdate{{Name: p.cfg.ID, State: st}})
}

func (s *BambuService) maybeNotify(p *bambuPrinter, prev, cur BambuPrinterState) {
	if s.push == nil {
		return
	}
	wasActive := prev.State == "printing" || prev.State == "paused"
	if !wasActive {
		return
	}

	name := cur.Filename
	if name == "" {
		name = "print"
	}
	started := ""
	if cur.StartedAt > 0 {
		started = " Started " + formatBambuTime(cur.StartedAt) + "."
	}
	now := formatBambuTime(time.Now().UnixMilli())
	switch cur.State {
	case "finished":
		s.push.SendPrintNotification(p.cfg.ID, prev.TaskID, s.notifyTitle("Print finished ✅"),
			fmt.Sprintf("%q finished at %s.%s", name, now, started))
	case "failed":
		detail := ""
		if cur.ErrorCode != "" && cur.ErrorCode != "0" {
			detail = fmt.Sprintf(" (error %s)", cur.ErrorCode)
		}
		s.push.SendPrintNotification(p.cfg.ID, prev.TaskID, s.notifyTitle("Print failed ❌"),
			fmt.Sprintf("%q failed%s at %s.%s", name, detail, now, started))
	}
}

// formatBambuTime renders a unix-millis timestamp as a short local datetime.
func formatBambuTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04")
}

func (s *BambuService) notifyTitle(base string) string {
	if s.footerName != "" {
		return s.footerName + " · " + base
	}
	return base
}

// watchdogLoop marks printers offline when no message has arrived recently.
func (s *BambuService) watchdogLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		for _, p := range s.order {
			p.mu.Lock()
			stale := p.hasState && p.last.Online && time.Since(p.lastMsg) > bambuOfflineThreshold
			var st BambuPrinterState
			if stale {
				st = p.last
				st.State = "offline"
				st.Online = false
				p.last = st
			}
			p.mu.Unlock()
			if stale {
				s.publish(p, st)
			}
		}
	}
}

// CurrentTaskID returns the task id of the print currently active on the given
// printer (empty if unknown). Used when recording a notification subscription.
func (s *BambuService) CurrentTaskID(printerID string) string {
	p, ok := s.printers[printerID]
	if !ok {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last.TaskID
}

// deriveBambuState maps the merged "print" object into the small state struct.
func deriveBambuState(full map[string]any) BambuPrinterState {
	st := BambuPrinterState{}

	gs := strings.ToUpper(bambuStr(full, "gcode_state"))
	switch gs {
	case "RUNNING", "SLICING", "PREPARE":
		st.State = "printing"
	case "PAUSE":
		st.State = "paused"
	case "FINISH":
		st.State = "finished"
	case "FAILED":
		st.State = "failed"
	default:
		st.State = "idle"
	}

	st.Progress = bambuInt(full, "mc_percent")
	st.RemainingTime = bambuInt(full, "mc_remaining_time")
	st.ErrorCode = bambuStr(full, "mc_print_error_code")
	st.TaskID = bambuStr(full, "task_id")
	st.LayerNum = bambuInt(full, "layer_num")
	st.TotalLayerNum = bambuInt(full, "total_layer_num")
	// gcode_start_time is unix seconds (as a string); expose it as unix millis.
	if start := bambuInt64(full, "gcode_start_time"); start > 0 {
		st.StartedAt = start * 1000
	}
	st.NozzleTemp = bambuFloat(full, "nozzle_temper")
	st.NozzleTarget = bambuFloat(full, "nozzle_target_temper")
	st.BedTemp = bambuFloat(full, "bed_temper")
	st.BedTarget = bambuFloat(full, "bed_target_temper")
	st.ChamberTemp = bambuFloat(full, "chamber_temper")

	// A non-zero print_error while printing indicates a hard failure.
	if st.State == "printing" && bambuInt(full, "print_error") != 0 {
		st.State = "failed"
	}

	// Filename: prefer the human-readable subtask name, else the gcode file base.
	name := bambuStr(full, "subtask_name")
	if name == "" {
		if gf := bambuStr(full, "gcode_file"); gf != "" {
			base := path.Base(gf)
			name = strings.TrimSuffix(base, path.Ext(base))
		}
	}
	st.Filename = name

	return st
}

// bambuStr reads a string value; Bambu encodes some numbers as strings.
func bambuStr(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// bambuInt reads an integer, tolerating both numeric and string encodings.
func bambuInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return int(f)
		}
	}
	return 0
}

// bambuInt64 reads a 64-bit integer, tolerating both numeric and string encodings.
func bambuInt64(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return 0
}

// bambuFloat reads a float, tolerating both numeric and string encodings.
func bambuFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}
