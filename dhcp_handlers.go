package main

import (
	"net"
	"sort"

	"github.com/gofiber/fiber/v2"
)

// dhcpConnectionResponse describes how a device is attached to the network.
// Type is "wired", "wifi" or "" (unknown).
type dhcpConnectionResponse struct {
	Type       string `json:"type"`
	Source     string `json:"source,omitempty"`
	SwitchName string `json:"switch_name,omitempty"`
	Port       string `json:"port,omitempty"`
	ApName     string `json:"ap_name,omitempty"`
	Ssid       string `json:"ssid,omitempty"`
	Rssi       int    `json:"rssi,omitempty"`
	SignalDbm  int    `json:"signal_dbm,omitempty"`
}

type dhcpLeaseResponse struct {
	MacAddress string                  `json:"mac_address"`
	IPAddress  string                  `json:"ip_address"`
	Hostname   string                  `json:"hostname"`
	Comment    string                  `json:"comment"`
	Server     string                  `json:"server"`
	Dynamic    bool                    `json:"dynamic"`
	Vendor     string                  `json:"vendor"`
	FirstSeen  int64                   `json:"first_seen"`
	LeaseStart int64                   `json:"lease_start"`
	LastSeen   int64                   `json:"last_seen"`
	Online     bool                    `json:"online"`
	Connection *dhcpConnectionResponse `json:"connection"`
}

type dhcpLeasesResponse struct {
	Leases         []dhcpLeaseResponse `json:"leases"`
	LastScrapeTime int64               `json:"last_scrape_time"`
	ScrapeError    string              `json:"scrape_error"`
}

// handleDhcpLeases serves the DHCP lease list, filtered to the subnets the
// authenticated user's OIDC groups are permitted to see. Requires AuthMiddleware.
func handleDhcpLeases(c *fiber.Ctx) error {
	if dhcpService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "DHCP tracking not configured"})
	}

	groups, err := getUserGroups(c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to parse claims"})
	}
	allowed := dhcpService.AllowedNetsForGroups(groups)

	var rows []DhcpLeaseModel
	if err := dhcpService.db.Order("ip_address asc").Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	dhcpService.mu.RLock()
	lastScrape := dhcpService.lastScrapeTime
	scrapeErr := dhcpService.lastScrapeErr
	dhcpService.mu.RUnlock()

	offlineMs := dhcpService.offlineThreshold.Milliseconds()
	now := CurrentTimestampMillis()

	leases := make([]dhcpLeaseResponse, 0, len(rows))
	for _, row := range rows {
		ip := net.ParseIP(row.IPAddress)
		if ip == nil || !ipInAny(ip, allowed) {
			continue
		}

		wired, wifi := dhcpService.ConnectionFor(row.MacAddress)
		var conn *dhcpConnectionResponse
		switch {
		case wifi != nil:
			conn = &dhcpConnectionResponse{
				Type:      "wifi",
				Source:    wifi.SourceName,
				ApName:    wifi.ApName,
				Ssid:      wifi.Ssid,
				Rssi:      wifi.Rssi,
				SignalDbm: wifi.SignalDbm,
			}
		case wired != nil:
			conn = &dhcpConnectionResponse{
				Type:       "wired",
				SwitchName: wired.SwitchName,
				Port:       wired.Port,
			}
		}

		leases = append(leases, dhcpLeaseResponse{
			MacAddress: row.MacAddress,
			IPAddress:  row.IPAddress,
			Hostname:   row.Hostname,
			Comment:    row.Comment,
			Server:     row.Server,
			Dynamic:    row.Dynamic,
			Vendor:     dhcpService.oui.Lookup(row.MacAddress),
			FirstSeen:  row.FirstSeen,
			LeaseStart: row.LeaseStart,
			LastSeen:   row.LastSeen,
			Online:     now-row.LastSeen <= offlineMs,
			Connection: conn,
		})
	}

	// Stable numeric-ish ordering by IP (DB string sort puts 10.12.20.9 after
	// 10.12.20.10); re-sort by IP bytes for a natural order.
	sort.SliceStable(leases, func(i, j int) bool {
		return ipLess(leases[i].IPAddress, leases[j].IPAddress)
	})

	return c.JSON(dhcpLeasesResponse{
		Leases:         leases,
		LastScrapeTime: lastScrape,
		ScrapeError:    scrapeErr,
	})
}

func ipInAny(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func ipLess(a, b string) bool {
	ipa, ipb := net.ParseIP(a), net.ParseIP(b)
	if ipa == nil || ipb == nil {
		return a < b
	}
	a16, b16 := ipa.To16(), ipb.To16()
	for i := range a16 {
		if a16[i] != b16[i] {
			return a16[i] < b16[i]
		}
	}
	return false
}
