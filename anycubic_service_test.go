package main

import (
	"bufio"
	"bytes"
	"image/png"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeAnycubic is a minimal uart-wifi printer: it answers sysinfo/getstatus,
// getfile and the preview commands on a local TCP port.
type fakeAnycubic struct {
	t  *testing.T
	ln net.Listener

	mu            sync.Mutex
	status        string   // full getstatus response without ",end"
	files         []string // getfile entries
	noPreview     map[string]bool
	previewW      int
	previewH      int
	preview2Calls int
	printing      bool // set when status is "print"; previews then fail the test
}

func newFakeAnycubic(t *testing.T) *fakeAnycubic {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeAnycubic{t: t, ln: ln, status: "getstatus,stop\r\n", previewW: 4, previewH: 3}
	t.Cleanup(func() { ln.Close() })
	go f.serve()
	return f
}

func (f *fakeAnycubic) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeAnycubic) setStatus(s string) {
	f.mu.Lock()
	f.status = s
	f.printing = strings.HasPrefix(s, "getstatus,print")
	f.mu.Unlock()
}

func (f *fakeAnycubic) serve() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(c)
	}
}

func (f *fakeAnycubic) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		// Commands are not newline terminated; read whatever arrived.
		buf := make([]byte, 256)
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := r.Read(buf)
		if err != nil {
			return
		}
		cmd := strings.TrimSuffix(string(buf[:n]), ",end")
		name, arg, _ := strings.Cut(cmd, ",")
		f.mu.Lock()
		switch name {
		case "sysinfo":
			c.Write([]byte("sysinfo,Photon Mono X,V3.5.4,203535394134500A,Hackerspace IoT,end"))
		case "getstatus":
			c.Write([]byte(f.status + ",end"))
		case "getfile":
			c.Write([]byte("getfile," + strings.Join(f.files, ",") + ",end"))
		case "getPreview1":
			if f.printing {
				f.t.Errorf("getPreview1 sent while printing")
			}
			if f.noPreview[arg] {
				c.Write([]byte("getPreview1,ERROR2,end"))
			} else {
				c.Write([]byte("getPreview1," + arg + ",4,3,172,end"))
			}
		case "getPreview2":
			f.preview2Calls++
			px := make([]byte, f.previewW*f.previewH*2)
			px[0], px[1] = 0x00, 0xf8 // first pixel pure red (0xF800 little-endian)
			c.Write(px)
		}
		f.mu.Unlock()
	}
}

func TestParseAnycubicStatus(t *testing.T) {
	st, err := parseAnycubicStatus(splitAnycubicFields(
		"getstatus,print,dodatki_Plate 1.pwmx/64.pwmx,918,12,110,600,11572,~37.12mL,UV,37.12,0.05,0"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "print" || st.ExternalName != "dodatki_Plate 1.pwmx" || st.InternalName != "64.pwmx" ||
		st.TotalLayers != 918 || st.Progress != 12 || st.CurrentLayer != 110 || st.ElapsedSeconds != 600 ||
		st.RemainingSeconds != 11572 || st.ResinVolume != 37.12 || st.ExposureMode != "UV" || st.LayerHeight != 0.05 {
		t.Errorf("unexpected parse: %+v", st)
	}

	// Commas inside the file name must not shift the numeric fields.
	st, _ = parseAnycubicStatus(splitAnycubicFields(
		"getstatus,print,a,b.pwmx/5.pwmx,100,50,50,1,2,~1mL,UV,1.0,0.05,0"))
	if st.ExternalName != "a,b.pwmx" || st.InternalName != "5.pwmx" || st.TotalLayers != 100 {
		t.Errorf("comma file name: %+v", st)
	}

	st, _ = parseAnycubicStatus(splitAnycubicFields("getstatus,stop\r\n"))
	if st.Status != "stop" || st.InternalName != "" {
		t.Errorf("stop: %+v", st)
	}
}

func TestAnycubicDisplayName(t *testing.T) {
	cases := map[string]string{
		"Glowy_2.5S.pwmx": "Glowy_2.5S",
		"Smart Meter IR Reader-SMT 0924 - SN154BE09DE42utj": "Smart Meter IR Reader-SMT 0924 - SN154BE09DE42utj",
		"_supported_Demeter_nyx_hunter_5_torso_(repaired).": "_supported_Demeter_nyx_hunter_5_torso_(repaired)",
		"x.pwmb": "x",
	}
	for in, want := range cases {
		if got := anycubicDisplayName(in); got != want {
			t.Errorf("anycubicDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEncodeRGB565PNG(t *testing.T) {
	raw := []byte{0x00, 0xf8, 0xe0, 0x07, 0x1f, 0x00, 0xff, 0xff} // red, green, blue, white
	out, err := encodeRGB565PNG(raw, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	want := [][3]uint32{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255}}
	for x, w := range want {
		r, g, b, _ := img.At(x, 0).RGBA()
		if r>>8 != w[0] || g>>8 != w[1] || b>>8 != w[2] {
			t.Errorf("pixel %d = %d,%d,%d want %v", x, r>>8, g>>8, b>>8, w)
		}
	}
}

func newTestAnycubicService(t *testing.T, port int) (*AnycubicService, *anycubicPrinter, *VdevManager) {
	vdev := NewVdevManager()
	cfg := &Config{AnycubicPrinters: []AnycubicPrinterConfig{{ID: "anycubic/mono", Host: "127.0.0.1", Port: port}}}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1) // every pooled connection would get its own :memory: DB
	if err := AutoMigrateModels(db); err != nil {
		t.Fatal(err)
	}
	s := NewAnycubicService(cfg, vdev, nil, db)
	return s, s.printers["anycubic/mono"], vdev
}

func TestAnycubicPollLifecycle(t *testing.T) {
	fake := newFakeAnycubic(t)
	fake.files = []string{"dodatki_Plate 1.pwmx/64.pwmx", "x.pwmx/8.pwmx"}
	s, p, _ := newTestAnycubicService(t, fake.port())
	now := time.Unix(1_700_000_000, 0)

	// First poll of an idle printer syncs the previews of all files.
	s.poll(p, now)
	if p.cur.State != "idle" || !p.cur.Online || p.cur.Model != "Photon Mono X" || p.cur.Type != PrinterTypeAnycubic {
		t.Fatalf("idle state: %+v", p.cur)
	}
	if fake.preview2Calls != 2 {
		t.Fatalf("expected 2 previews synced, got %d", fake.preview2Calls)
	}

	// A print starts: started_at is back-dated by the elapsed time and the
	// thumbnail comes from the cache (no preview commands while printing).
	fake.setStatus("getstatus,print,dodatki_Plate 1.pwmx/64.pwmx,918,10,92,600,6000,~37.12mL,UV,37.12,0.05,0")
	now = now.Add(30 * time.Second)
	s.poll(p, now)
	if p.cur.State != "printing" || p.cur.Filename != "dodatki_Plate 1" || p.cur.RemainingTime != 100 || p.cur.ElapsedTime != 10 {
		t.Fatalf("printing state: %+v", p.cur)
	}
	if want := now.Add(-600 * time.Second).UnixMilli(); p.cur.StartedAt != want {
		t.Errorf("StartedAt = %d, want %d", p.cur.StartedAt, want)
	}
	if !p.cur.HasThumbnail {
		t.Error("expected thumbnail after first printing poll")
	}
	thumb, err := s.CurrentThumbnail("anycubic/mono")
	if err != nil || !bytes.HasPrefix(thumb, pngMagic) {
		t.Fatalf("CurrentThumbnail: %v", err)
	}
	taskID := p.cur.TaskID
	startedAt := p.cur.StartedAt

	// Progress continues: same task, no second preview download.
	fake.setStatus("getstatus,print,dodatki_Plate 1.pwmx/64.pwmx,918,99,915,11500,30,~37.12mL,UV,37.12,0.05,0")
	now = now.Add(30 * time.Second)
	s.poll(p, now)
	if p.cur.TaskID != taskID || p.cur.StartedAt != startedAt || fake.preview2Calls != 2 {
		t.Errorf("task changed or preview re-fetched: task %q->%q, preview2 calls %d", taskID, p.cur.TaskID, fake.preview2Calls)
	}

	// Printer goes back to stop near the end: finished.
	fake.setStatus("getstatus,stop\r\n")
	now = now.Add(30 * time.Second)
	s.poll(p, now)
	if p.cur.State != "finished" || p.cur.Progress != 100 || p.cur.FinishedAt != now.UnixMilli() || p.cur.Filename != "dodatki_Plate 1" {
		t.Fatalf("finished state: %+v", p.cur)
	}
	// The finished state sticks while the printer stays stopped.
	s.poll(p, now.Add(30*time.Second))
	if p.cur.State != "finished" {
		t.Errorf("expected finished to persist, got %q", p.cur.State)
	}
}

func TestAnycubicStoppedEarlyIsFailed(t *testing.T) {
	fake := newFakeAnycubic(t)
	s, p, _ := newTestAnycubicService(t, fake.port())
	now := time.Unix(1_700_000_000, 0)

	fake.setStatus("getstatus,print,x.pwmx/8.pwmx,794,20,160,1200,5000,~97mL,UV,97.26,0.05,0")
	s.poll(p, now)
	fake.setStatus("getstatus,stop\r\n")
	s.poll(p, now.Add(30*time.Second))
	if p.cur.State != "failed" || !strings.Contains(p.cur.PrintErrorText, "160/794") {
		t.Fatalf("expected failed with layer info, got %+v", p.cur)
	}
}

func TestAnycubicOffline(t *testing.T) {
	fake := newFakeAnycubic(t)
	s, p, vdev := newTestAnycubicService(t, fake.port())
	s.poll(p, time.Now())
	fake.ln.Close()

	s.poll(p, time.Now())
	if st := vdevState(vdev, "anycubic/mono"); !st.Online {
		t.Error("a single failed poll should not mark the printer offline")
	}
	s.poll(p, time.Now())
	if st := vdevState(vdev, "anycubic/mono"); st.Online || st.State != "offline" {
		t.Errorf("expected offline after repeated failures, got %+v", st)
	}
}

func vdevState(m *VdevManager, id string) AnycubicPrinterState {
	for _, d := range m.Devices() {
		if d.ID == id {
			return d.State.(AnycubicPrinterState)
		}
	}
	return AnycubicPrinterState{}
}

func TestAnycubicPreviewSync(t *testing.T) {
	fake := newFakeAnycubic(t)
	fake.files = []string{"a.pwmx/1.pwmx", "b.pwmx/2.pwmx"}
	fake.noPreview = map[string]bool{"2.pwmx": true}
	s, p, _ := newTestAnycubicService(t, fake.port())
	now := time.Unix(1_700_000_000, 0)

	s.poll(p, now) // poll 1: sync
	if fake.preview2Calls != 1 || !s.hasThumbnail(p.cfg.ID, "a.pwmx/1.pwmx") {
		t.Fatalf("expected a.pwmx cached, preview2 calls %d", fake.preview2Calls)
	}

	// A new file appears and an old one is deleted; picked up only on poll 6.
	fake.mu.Lock()
	fake.files = []string{"b.pwmx/2.pwmx", "c.pwmx/3.pwmx"}
	fake.mu.Unlock()
	for i := 2; i <= 5; i++ {
		s.poll(p, now.Add(time.Duration(i)*30*time.Second))
	}
	if fake.preview2Calls != 1 {
		t.Fatalf("synced before the 5th poll: %d preview2 calls", fake.preview2Calls)
	}
	s.poll(p, now.Add(6*30*time.Second))
	if !s.hasThumbnail(p.cfg.ID, "c.pwmx/3.pwmx") {
		t.Error("expected c.pwmx cached after sync")
	}
	if s.hasThumbnail(p.cfg.ID, "a.pwmx/1.pwmx") {
		t.Error("expected deleted a.pwmx to be pruned")
	}
	// b.pwmx (no preview on the printer) is not retried.
	if fake.preview2Calls != 2 {
		t.Errorf("preview2 calls = %d, want 2", fake.preview2Calls)
	}

	// No sync while printing, even on a sync poll.
	fake.setStatus("getstatus,print,c.pwmx/3.pwmx,100,10,10,60,600,~1mL,UV,1.0,0.05,0")
	for i := 7; i <= 12; i++ {
		s.poll(p, now.Add(time.Duration(i)*30*time.Second))
	}
	if !p.cur.HasThumbnail {
		t.Error("expected cached thumbnail for the running print")
	}
}
