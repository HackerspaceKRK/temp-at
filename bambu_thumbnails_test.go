package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// sample3mf is a real Bambu .gcode.3mf used to exercise the zip/_rels extraction
// without a live printer. Skipped when not present.
func sample3mfPath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	p := filepath.Join(home, "Asdf", "Modelo cortado LADO 1.gcode.3mf")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("sample 3mf not available (%s): %v", p, err)
	}
	return p
}

func TestBambuThumbnailExtraction(t *testing.T) {
	path := sample3mfPath(t)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open sample: %v", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("stat sample: %v", err)
	}

	// *os.File is itself an io.ReaderAt, so it stands in for ftpReaderAt here.
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	target, err := bambuThumbnailTarget(zr)
	if err != nil {
		t.Fatalf("resolve thumbnail target: %v", err)
	}
	if target == "" {
		t.Fatal("empty thumbnail target")
	}
	t.Logf("thumbnail entry: %s", target)

	png, err := readZipFile(zr, target)
	if err != nil {
		t.Fatalf("read thumbnail %q: %v", target, err)
	}
	if len(png) == 0 {
		t.Fatal("thumbnail is empty")
	}
	if !isPNG(png) {
		t.Fatalf("thumbnail does not start with PNG magic; got % x", png[:min(8, len(png))])
	}
	t.Logf("extracted %d-byte PNG thumbnail", len(png))
}

func TestBambuSanitizeFilename(t *testing.T) {
	got := bambuSanitizeFilename(`BLV - AMS / AMS 2 Riser P2S / X2D / X1C / P1S v4`)
	want := `BLV - AMS _ AMS 2 Riser P2S _ X2D _ X1C _ P1S v4`
	if got != want {
		t.Errorf("sanitize = %q, want %q", got, want)
	}
	if s := bambuSanitizeFilename("plain-name v2"); s != "plain-name v2" {
		t.Errorf("legal name changed: %q", s)
	}
}

func TestBambu3mfCandidates_WithSlashes(t *testing.T) {
	cands := bambu3mfCandidates("a/b")
	found := false
	for _, c := range cands {
		if c == "/cache/a_b.gcode.3mf" {
			found = true
		}
	}
	if !found {
		t.Errorf("sanitized cache candidate missing, got %v", cands)
	}
}

func TestBambu3mfNameMatches(t *testing.T) {
	want := "BLV - AMS / AMS v4.gcode.3mf"
	cases := map[string]bool{
		"BLV - AMS _ AMS v4.gcode.3mf":                 true,  // underscore replacement
		"BLV - AMS - AMS v4.gcode.3mf":                 true,  // any replacement char
		"BLV - AMS / AMS v4.gcode.3mf":                 true,  // exact
		"110_BLV+-+AMS+Riser+X1C+P1P+P1S+v4.gcode.3mf": true,  // printer-mapped variant
		"BLV - AMS _ AMS v5.gcode.3mf":                 false, // legal char differs
		"other.gcode.3mf":                              false, // different length
	}
	for entry, wantMatch := range cases {
		if got := bambu3mfNameMatches(entry, want); got != wantMatch {
			t.Errorf("match(%q) = %v, want %v", entry, got, wantMatch)
		}
	}
}
