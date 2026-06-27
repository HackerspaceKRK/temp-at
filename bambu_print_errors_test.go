package main

import "testing"

func TestFormatBambuPrintError(t *testing.T) {
	cases := map[int]string{
		0x0300800A: "0300800A",
		0x03008003: "03008003",
		0:          "00000000",
	}
	for in, want := range cases {
		if got := formatBambuPrintError(in); got != want {
			t.Errorf("formatBambuPrintError(%#x) = %q, want %q", in, got, want)
		}
	}
}

func TestBambuPrintErrorText(t *testing.T) {
	if got := bambuPrintErrorText("0300800A"); got == "" {
		t.Fatal("expected text for 0300800A (filament pile-up), got empty")
	}
	if got := bambuPrintErrorText("FFFFFFFF"); got != "" {
		t.Errorf("expected empty text for unknown code, got %q", got)
	}
}
