package main

import "testing"

func TestOuiLookup(t *testing.T) {
	db, err := LoadEmbeddedOuiDB()
	if err != nil {
		t.Fatalf("LoadEmbeddedOuiDB: %v", err)
	}

	cases := []struct {
		name    string
		mac     string
		wantSub string // substring expected in the vendor name
	}{
		{"24-bit cisco", "00:00:0c:11:22:33", "Cisco"},
		{"24-bit case-insensitive", "00000C112233", "Cisco"},
		{"24-bit dash-separated", "00-00-0C-11-22-33", "Cisco"},
		{"28-bit shinko", "00:55:da:01:22:33", "Shinko"},
		{"36-bit converging", "00:1b:c5:00:01:22", "Converging"},
		{"unknown returns empty", "ff:ff:ff:ff:ff:ff", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := db.Lookup(tc.mac)
			if tc.wantSub == "" {
				if got != "" {
					t.Fatalf("Lookup(%q) = %q, want empty", tc.mac, got)
				}
				return
			}
			if !contains(got, tc.wantSub) {
				t.Fatalf("Lookup(%q) = %q, want substring %q", tc.mac, got, tc.wantSub)
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
