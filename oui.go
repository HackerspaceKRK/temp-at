package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"strings"
)

// manufGz is the Wireshark "manuf" OUI database, gzipped.
//
// To refresh: download https://www.wireshark.org/download/automated/data/manuf
// and run `gzip -9 -c manuf > manuf.gz` in the repo root.
//
//go:embed manuf.gz
var manufGz []byte

// OuiDB maps MAC address prefixes to vendor names. The IEEE allocates blocks of
// three sizes (24, 28 and 36 bits); a lookup probes the longest first so a more
// specific allocation wins over the enclosing 24-bit block.
type OuiDB struct {
	by24 map[string]string // 6 hex nibbles
	by28 map[string]string // 7 hex nibbles
	by36 map[string]string // 9 hex nibbles
}

// LoadEmbeddedOuiDB parses the embedded gzipped manuf file into an OuiDB.
func LoadEmbeddedOuiDB() (*OuiDB, error) {
	gz, err := gzip.NewReader(bytes.NewReader(manufGz))
	if err != nil {
		return nil, fmt.Errorf("opening embedded manuf.gz: %w", err)
	}
	defer gz.Close()

	db := &OuiDB{
		by24: make(map[string]string),
		by28: make(map[string]string),
		by36: make(map[string]string),
	}

	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Tab-separated: prefix \t short name \t [full name]
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		prefixField := strings.TrimSpace(fields[0])
		// Prefer the full vendor name (3rd column); fall back to the short name.
		name := ""
		if len(fields) >= 3 {
			name = strings.TrimSpace(fields[2])
		}
		if name == "" {
			name = strings.TrimSpace(fields[1])
		}
		if name == "" {
			continue
		}

		prefix, bits := parseManufPrefix(prefixField)
		if prefix == "" {
			continue
		}
		switch bits {
		case 24:
			db.by24[prefix] = name
		case 28:
			db.by28[prefix] = name
		case 36:
			db.by36[prefix] = name
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading manuf: %w", err)
	}
	return db, nil
}

// parseManufPrefix turns a manuf prefix field ("00:00:01" or "00:55:DA:00/28")
// into a lowercase hex-nibble string truncated to the significant bits, plus the
// bit width. Returns ("", 0) for anything it can't interpret.
func parseManufPrefix(field string) (string, int) {
	bits := 24
	if slash := strings.IndexByte(field, '/'); slash >= 0 {
		switch field[slash+1:] {
		case "28":
			bits = 28
		case "36":
			bits = 36
		default:
			return "", 0
		}
		field = field[:slash]
	}
	hex := strings.ToLower(strings.ReplaceAll(field, ":", ""))
	nibbles := bits / 4
	if len(hex) < nibbles {
		return "", 0
	}
	return hex[:nibbles], bits
}

// Lookup returns the vendor name for a MAC address, or "" if unknown. It checks
// the 36-, 28- then 24-bit blocks so the most specific allocation wins.
func (d *OuiDB) Lookup(mac string) string {
	hex := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", "")) //nolint:gocritic
	if len(hex) < 6 {
		return ""
	}
	if len(hex) >= 9 {
		if v, ok := d.by36[hex[:9]]; ok {
			return v
		}
	}
	if len(hex) >= 7 {
		if v, ok := d.by28[hex[:7]]; ok {
			return v
		}
	}
	if v, ok := d.by24[hex[:6]]; ok {
		return v
	}
	return ""
}
