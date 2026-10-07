package main

import (
	"bytes"
	"testing"
	"time"
)

func TestMembershipActive(t *testing.T) {
	prev := ConfigInstance
	defer func() { ConfigInstance = prev }()
	ConfigInstance = &Config{Oidc: &OidcConfig{MembershipExpirationTimestampClaim: "exp_at"}}

	future := float64(time.Now().Add(time.Hour).Unix())
	past := float64(time.Now().Add(-time.Hour).Unix())
	cases := []struct {
		name   string
		claims map[string]interface{}
		want   bool
	}{
		{"future", map[string]interface{}{"exp_at": future}, true},
		{"past", map[string]interface{}{"exp_at": past}, false},
		{"string future", map[string]interface{}{"exp_at": "99999999999"}, true},
		{"missing", map[string]interface{}{}, false},
		{"garbage", map[string]interface{}{"exp_at": "soon"}, false},
	}
	for _, tc := range cases {
		if got := membershipActive(tc.claims); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}

	ConfigInstance.Oidc.MembershipExpirationTimestampClaim = ""
	if !membershipActive(map[string]interface{}{}) {
		t.Errorf("unconfigured claim should skip the check")
	}
}

func TestEmergencyEntryConfigTemplates(t *testing.T) {
	cfg := &Config{EmergencyEntry: &EmergencyEntryConfig{
		AllowedSubnets: []string{"10.0.0.0/8"},
		Doors: []EmergencyEntryDoor{
			{ID: "front", URL: "http://door/open?u={{urlquery .Username}}"},
		},
	}}
	validateEmergencyEntryConfig(cfg, "test")

	if cfg.EmergencyEntry.Doors[0].Method != "POST" {
		t.Errorf("default method = %q, want POST", cfg.EmergencyEntry.Doors[0].Method)
	}
	var buf bytes.Buffer
	if err := emergencyEntryTemplates["front"].Execute(&buf, struct{ Username string }{"a b&c"}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "http://door/open?u=a+b%26c"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
	if len(emergencyEntryNets) != 1 {
		t.Errorf("expected 1 parsed subnet, got %d", len(emergencyEntryNets))
	}
}
