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
