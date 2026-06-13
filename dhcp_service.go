package main

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	defaultScrapeInterval   = time.Minute
	defaultOfflineThreshold = 3 * time.Minute
)

// DhcpService periodically scrapes the configured router's DHCP leases, persists
// their lifecycle (first-seen / continuous-online-since / last-seen) in SQLite,
// and keeps an in-memory snapshot of live connection info (switch port for wired
// devices, AP/SSID/RSSI for WiFi) used to enrich the API responses.
type DhcpService struct {
	cfg *DhcpConfig
	db  *gorm.DB
	oui *OuiDB

	leaseSource  DhcpLeaseSource
	wiredSources []WiredPortSource
	wifiSources  []WifiClientSource

	scrapeInterval   time.Duration
	offlineThreshold time.Duration
	pruneAfter       time.Duration // 0 = never prune

	// Parsed access-control CIDRs (validated at config load).
	groupNets   map[string][]*net.IPNet
	defaultNets []*net.IPNet

	mu             sync.RWMutex
	wiredByMac     map[string]WiredPortInfo
	wifiByMac      map[string]WifiClientInfo
	lastScrapeTime int64 // millis of last successful lease scrape
	// prevSuccessTime is the lease-scrape success time before the current one.
	// Used to distinguish a device that was genuinely absent across consecutive
	// successful scrapes (reset LeaseStart) from scraper downtime / a restart
	// (do not reset).
	prevSuccessTime int64
	lastScrapeErr   string
}

// NewDhcpService wires up sources, the OUI database and parsed access CIDRs.
func NewDhcpService(cfg *Config, db *gorm.DB) (*DhcpService, error) {
	dcfg := cfg.Dhcp

	leaseSource, err := NewDhcpLeaseSource(dcfg.Router)
	if err != nil {
		return nil, err
	}

	wiredSources := make([]WiredPortSource, 0, len(dcfg.WiredSources))
	for _, wc := range dcfg.WiredSources {
		src, err := NewWiredPortSource(wc)
		if err != nil {
			return nil, err
		}
		wiredSources = append(wiredSources, src)
	}

	wifiSources := make([]WifiClientSource, 0, len(dcfg.WifiSources))
	for _, wc := range dcfg.WifiSources {
		src, err := NewWifiClientSource(wc)
		if err != nil {
			return nil, err
		}
		wifiSources = append(wifiSources, src)
	}

	oui, err := LoadEmbeddedOuiDB()
	if err != nil {
		return nil, err
	}

	s := &DhcpService{
		cfg:              dcfg,
		db:               db,
		oui:              oui,
		leaseSource:      leaseSource,
		wiredSources:     wiredSources,
		wifiSources:      wifiSources,
		scrapeInterval:   parseDurationOr(dcfg.ScrapeInterval, defaultScrapeInterval),
		offlineThreshold: parseDurationOr(dcfg.OfflineThreshold, defaultOfflineThreshold),
		pruneAfter:       parseDurationOr(dcfg.PruneAfter, 0),
		groupNets:        make(map[string][]*net.IPNet),
		wiredByMac:       map[string]WiredPortInfo{},
		wifiByMac:        map[string]WifiClientInfo{},
	}

	// CIDRs are validated at config load, so parse errors here are not expected.
	for group, cidrs := range dcfg.Access.GroupCidrs {
		s.groupNets[group] = parseCIDRs(cidrs)
	}
	s.defaultNets = parseCIDRs(dcfg.Access.DefaultCidrs)

	return s, nil
}

// Start launches the background scrape loop.
func (s *DhcpService) Start() {
	go s.scrapeLoop()
}

func (s *DhcpService) scrapeLoop() {
	ticker := time.NewTicker(s.scrapeInterval)
	defer ticker.Stop()
	for {
		s.scrapeOnce()
		<-ticker.C
	}
}

// scrapeOnce performs one full scrape: leases (persisted) plus wired/wifi
// enrichment (in-memory). Each part fails independently so a flaky enrichment
// source never blocks lease tracking.
func (s *DhcpService) scrapeOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), s.scrapeInterval/2+5*time.Second)
	defer cancel()

	leases, err := s.leaseSource.FetchLeases(ctx)
	if err != nil {
		log.Printf("[dhcp] lease scrape failed: %v", err)
		s.mu.Lock()
		s.lastScrapeErr = err.Error()
		s.mu.Unlock()
	} else {
		if err := s.updateLeases(leases); err != nil {
			log.Printf("[dhcp] persisting leases failed: %v", err)
			s.mu.Lock()
			s.lastScrapeErr = err.Error()
			s.mu.Unlock()
		} else {
			s.mu.Lock()
			s.lastScrapeErr = ""
			s.mu.Unlock()
		}
	}

	// Wired enrichment: merge all switches into one map. Keep the previous map
	// for any source that errors (stale info beats a flapping column).
	wired := make(map[string]WiredPortInfo)
	for _, src := range s.wiredSources {
		hosts, err := src.FetchHosts(ctx)
		if err != nil {
			log.Printf("[dhcp] wired source %q failed: %v", src.Name(), err)
			continue
		}
		for mac, info := range hosts {
			if existing, dup := wired[mac]; dup {
				log.Printf("[dhcp] MAC %s seen on both %s:%s and %s:%s; using latter",
					mac, existing.SwitchName, existing.Port, info.SwitchName, info.Port)
			}
			wired[mac] = info
		}
	}

	wifi := make(map[string]WifiClientInfo)
	wifiOK := false
	for _, src := range s.wifiSources {
		clients, err := src.FetchClients(ctx)
		if err != nil {
			log.Printf("[dhcp] wifi source %q failed: %v", src.Name(), err)
			continue
		}
		wifiOK = true
		for mac, info := range clients {
			wifi[mac] = info
		}
	}

	s.mu.Lock()
	if len(s.wiredSources) > 0 && len(wired) > 0 {
		s.wiredByMac = wired
	}
	if wifiOK {
		s.wifiByMac = wifi
	}
	s.mu.Unlock()
}

// updateLeases persists one scrape's worth of bound leases inside a single
// transaction (SQLite is a single writer).
func (s *DhcpService) updateLeases(entries []DhcpLeaseEntry) error {
	now := CurrentTimestampMillis()
	offlineMs := s.offlineThreshold.Milliseconds()

	s.mu.RLock()
	prevSuccess := s.prevSuccessTime
	s.mu.RUnlock()

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Load existing rows once into a map keyed by "mac|server" to avoid a
		// per-lease SELECT (and the ErrRecordNotFound log noise that comes with
		// it on inserts).
		var existing []DhcpLeaseModel
		if err := tx.Find(&existing).Error; err != nil {
			return err
		}
		byKey := make(map[string]*DhcpLeaseModel, len(existing))
		for i := range existing {
			byKey[existing[i].MacAddress+"|"+existing[i].Server] = &existing[i]
		}

		for _, e := range entries {
			if e.Status != "bound" {
				continue
			}
			row, ok := byKey[e.MacAddress+"|"+e.Server]
			if !ok {
				newRow := DhcpLeaseModel{
					MacAddress: e.MacAddress,
					Server:     e.Server,
					IPAddress:  e.IPAddress,
					Hostname:   e.Hostname,
					Comment:    e.Comment,
					Dynamic:    e.Dynamic,
					FirstSeen:  now,
					LeaseStart: now,
					LastSeen:   now,
				}
				if err := tx.Create(&newRow).Error; err != nil {
					return err
				}
				continue
			}

			// Reset the continuous-online period only if the device was actually
			// observed as absent across successful scrapes (not merely because
			// the scraper itself was down). prevSuccess == 0 on first scrape.
			if prevSuccess > 0 && row.LastSeen < prevSuccess && now-row.LastSeen > offlineMs {
				row.LeaseStart = now
			}
			row.IPAddress = e.IPAddress
			row.Hostname = e.Hostname
			row.Comment = e.Comment
			row.Dynamic = e.Dynamic
			row.LastSeen = now
			if err := tx.Save(row).Error; err != nil {
				return err
			}
		}

		if s.pruneAfter > 0 {
			cutoff := now - s.pruneAfter.Milliseconds()
			if err := tx.Where("last_seen < ?", cutoff).Delete(&DhcpLeaseModel{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.prevSuccessTime = s.lastScrapeTime
	s.lastScrapeTime = now
	s.mu.Unlock()
	return nil
}

// ConnectionFor returns the live connection info for a MAC, preferring WiFi when
// a device appears in both maps (a wired hit for a wireless client is just the
// AP's own uplink port). Either or both may be nil.
func (s *DhcpService) ConnectionFor(mac string) (*WiredPortInfo, *WifiClientInfo) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var wired *WiredPortInfo
	var wifi *WifiClientInfo
	if w, ok := s.wifiByMac[mac]; ok {
		wifi = &w
		return nil, wifi
	}
	if w, ok := s.wiredByMac[mac]; ok {
		wired = &w
	}
	return wired, wifi
}

// AllowedNetsForGroups returns the CIDRs a user with the given OIDC groups may
// see: the union of the configured CIDRs for each matching group, or the default
// CIDRs when no group matches.
func (s *DhcpService) AllowedNetsForGroups(groups []string) []*net.IPNet {
	var nets []*net.IPNet
	matched := false
	for _, g := range groups {
		if gn, ok := s.groupNets[g]; ok {
			nets = append(nets, gn...)
			matched = true
		}
	}
	if !matched {
		return s.defaultNets
	}
	return nets
}

func parseDurationOr(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

func parseCIDRs(cidrs []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		nets = append(nets, n)
	}
	return nets
}
