package main

import (
	"context"
	"fmt"
)

// This file defines the vendor-neutral interfaces the DHCP scraper consumes and
// the factories that build concrete implementations from config. Adding support
// for a new switch or wireless vendor means implementing one of these interfaces
// and registering it by `kind` in the relevant factory below.

// DhcpLeaseEntry is one lease as reported by a router's DHCP server.
type DhcpLeaseEntry struct {
	MacAddress string
	IPAddress  string
	Hostname   string
	Comment    string
	Server     string // DHCP server name (e.g. "dhcp-users")
	Status     string // "bound", "waiting", ...
	Dynamic    bool
}

// DhcpLeaseSource fetches the current DHCP leases from a router.
type DhcpLeaseSource interface {
	FetchLeases(ctx context.Context) ([]DhcpLeaseEntry, error)
}

// WiredPortInfo locates a wired device: which switch and physical port it is on.
type WiredPortInfo struct {
	SwitchName string
	Port       string
}

// WiredPortSource reports MAC -> switch/port for devices connected by cable.
type WiredPortSource interface {
	Name() string
	FetchHosts(ctx context.Context) (map[string]WiredPortInfo, error) // keyed by normalized MAC
}

// WifiClientInfo describes how a wireless client is associated.
type WifiClientInfo struct {
	SourceName string
	ApName     string
	Ssid       string
	Rssi       int // controller-relative signal quality (vendor specific)
	SignalDbm  int // signal strength in dBm
}

// WifiClientSource reports MAC -> AP/SSID/signal for wireless clients.
type WifiClientSource interface {
	Name() string
	FetchClients(ctx context.Context) (map[string]WifiClientInfo, error) // keyed by normalized MAC
}

// NewDhcpLeaseSource builds the DHCP lease source selected by cfg.Kind.
func NewDhcpLeaseSource(cfg DhcpRouterConfig) (DhcpLeaseSource, error) {
	switch cfg.Kind {
	case "mikrotik":
		return NewMikrotikLeaseSource(cfg), nil
	default:
		return nil, fmt.Errorf("unknown dhcp router kind %q", cfg.Kind)
	}
}

// NewWiredPortSource builds the wired-port source selected by cfg.Kind.
func NewWiredPortSource(cfg DhcpWiredSourceConfig) (WiredPortSource, error) {
	switch cfg.Kind {
	case "mikrotik_bridge_host":
		return NewMikrotikBridgeHostSource(cfg), nil
	default:
		return nil, fmt.Errorf("unknown dhcp wired source kind %q", cfg.Kind)
	}
}

// NewWifiClientSource builds the wifi-client source selected by cfg.Kind.
func NewWifiClientSource(cfg DhcpWifiSourceConfig) (WifiClientSource, error) {
	switch cfg.Kind {
	case "unifi":
		return NewUnifiWifiSource(cfg), nil
	default:
		return nil, fmt.Errorf("unknown dhcp wifi source kind %q", cfg.Kind)
	}
}
