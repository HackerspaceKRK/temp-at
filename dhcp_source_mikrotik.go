package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	routeros "github.com/go-routeros/routeros/v3"
)

// mikrotikConn dials the RouterOS binary API on demand. We connect per scrape
// rather than holding a persistent client: at a one-per-minute cadence the cost
// is negligible and it makes the scraper automatically resilient to router
// reboots and idle-timeout disconnects without a reconnect state machine.
type mikrotikConn struct {
	address  string
	username string
	password string
	useTLS   bool
}

const mikrotikDialTimeout = 10 * time.Second

// run dials, executes one command, and closes the connection. The ctx bounds
// the command execution (dialing uses a fixed timeout).
func (m *mikrotikConn) run(ctx context.Context, sentence ...string) (*routeros.Reply, error) {
	var (
		client *routeros.Client
		err    error
	)
	if m.useTLS {
		client, err = routeros.DialTLSTimeout(m.address, m.username, m.password,
			&tls.Config{InsecureSkipVerify: true}, mikrotikDialTimeout)
	} else {
		client, err = routeros.DialTimeout(m.address, m.username, m.password, mikrotikDialTimeout)
	}
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", m.address, err)
	}
	defer client.Close()
	return client.RunContext(ctx, sentence...)
}

// MikrotikLeaseSource reads DHCP leases from a MikroTik router.
type MikrotikLeaseSource struct {
	conn mikrotikConn
}

func NewMikrotikLeaseSource(cfg DhcpRouterConfig) *MikrotikLeaseSource {
	return &MikrotikLeaseSource{conn: mikrotikConn{
		address:  cfg.Address,
		username: cfg.Username,
		password: cfg.Password,
		useTLS:   cfg.UseTLS,
	}}
}

func (s *MikrotikLeaseSource) FetchLeases(ctx context.Context) ([]DhcpLeaseEntry, error) {
	reply, err := s.conn.run(ctx, "/ip/dhcp-server/lease/print")
	if err != nil {
		return nil, fmt.Errorf("mikrotik lease print: %w", err)
	}

	leases := make([]DhcpLeaseEntry, 0, len(reply.Re))
	for _, sen := range reply.Re {
		m := sen.Map
		status := m["status"]
		if status != "bound" {
			// Only bound leases represent a device actually present on the network.
			continue
		}
		// Prefer the "active-*" fields (the address/MAC currently in use) and
		// fall back to the static configuration fields.
		ip := firstNonEmpty(m["active-address"], m["address"])
		mac := firstNonEmpty(m["active-mac-address"], m["mac-address"])
		if ip == "" || mac == "" {
			continue
		}
		leases = append(leases, DhcpLeaseEntry{
			MacAddress: NormalizeMac(mac),
			IPAddress:  ip,
			Hostname:   m["host-name"],
			Comment:    m["comment"],
			Server:     m["server"],
			Status:     status,
			Dynamic:    m["dynamic"] == "true",
		})
	}
	return leases, nil
}

// MikrotikBridgeHostSource reads the bridge host table to map MAC -> port.
type MikrotikBridgeHostSource struct {
	name             string
	conn             mikrotikConn
	ignoreInterfaces map[string]struct{}
}

func NewMikrotikBridgeHostSource(cfg DhcpWiredSourceConfig) *MikrotikBridgeHostSource {
	ignore := make(map[string]struct{}, len(cfg.IgnoreInterfaces))
	for _, iface := range cfg.IgnoreInterfaces {
		ignore[iface] = struct{}{}
	}
	return &MikrotikBridgeHostSource{
		name: cfg.Name,
		conn: mikrotikConn{
			address:  cfg.Address,
			username: cfg.Username,
			password: cfg.Password,
			useTLS:   cfg.UseTLS,
		},
		ignoreInterfaces: ignore,
	}
}

func (s *MikrotikBridgeHostSource) Name() string { return s.name }

func (s *MikrotikBridgeHostSource) FetchHosts(ctx context.Context) (map[string]WiredPortInfo, error) {
	reply, err := s.conn.run(ctx, "/interface/bridge/host/print")
	if err != nil {
		return nil, fmt.Errorf("mikrotik bridge host print: %w", err)
	}

	hosts := make(map[string]WiredPortInfo, len(reply.Re))
	for _, sen := range reply.Re {
		m := sen.Map
		if m["local"] == "true" {
			// The switch's own interface MACs, not connected devices.
			continue
		}
		iface := m["on-interface"]
		if iface == "" {
			continue
		}
		if _, ignored := s.ignoreInterfaces[iface]; ignored {
			continue
		}
		mac := m["mac-address"]
		if mac == "" {
			continue
		}
		hosts[NormalizeMac(mac)] = WiredPortInfo{SwitchName: s.name, Port: iface}
	}
	return hosts, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
