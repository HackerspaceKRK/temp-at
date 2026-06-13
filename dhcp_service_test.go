package main

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDhcpService(t *testing.T) *DhcpService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateModels(db); err != nil {
		t.Fatal(err)
	}
	return &DhcpService{
		db:               db,
		offlineThreshold: 3 * time.Minute,
		wiredByMac:       map[string]WiredPortInfo{},
		wifiByMac:        map[string]WifiClientInfo{},
	}
}

func boundEntry() DhcpLeaseEntry {
	return DhcpLeaseEntry{
		MacAddress: "aa:bb:cc:dd:ee:ff",
		Server:     "dhcp-users",
		IPAddress:  "10.0.0.5",
		Status:     "bound",
	}
}

func getLease(t *testing.T, s *DhcpService) DhcpLeaseModel {
	t.Helper()
	var row DhcpLeaseModel
	if err := s.db.First(&row).Error; err != nil {
		t.Fatalf("loading lease: %v", err)
	}
	return row
}

// First scrape inserts a row with FirstSeen == LeaseStart == LastSeen.
func TestUpdateLeases_InsertsNewLease(t *testing.T) {
	s := newTestDhcpService(t)
	if err := s.updateLeases([]DhcpLeaseEntry{boundEntry()}); err != nil {
		t.Fatal(err)
	}
	row := getLease(t, s)
	if row.FirstSeen == 0 || row.FirstSeen != row.LeaseStart || row.LeaseStart != row.LastSeen {
		t.Fatalf("expected FirstSeen==LeaseStart==LastSeen, got %+v", row)
	}
}

// A device absent across a successful scrape (gap > threshold) gets a fresh
// LeaseStart while FirstSeen is preserved.
func TestUpdateLeases_ResetsLeaseStartAfterAbsence(t *testing.T) {
	s := newTestDhcpService(t)
	now := CurrentTimestampMillis()
	origFirst := now - 60*60*1000  // an hour ago
	origStart := now - 60*60*1000
	s.db.Create(&DhcpLeaseModel{
		MacAddress: "aa:bb:cc:dd:ee:ff",
		Server:     "dhcp-users",
		IPAddress:  "10.0.0.5",
		FirstSeen:  origFirst,
		LeaseStart: origStart,
		LastSeen:   now - 10*60*1000, // last seen 10 min ago
	})
	// A successful scrape happened 5 min ago (after the device's last sighting),
	// so the device was genuinely absent then.
	s.prevSuccessTime = now - 5*60*1000

	if err := s.updateLeases([]DhcpLeaseEntry{boundEntry()}); err != nil {
		t.Fatal(err)
	}
	row := getLease(t, s)
	if row.FirstSeen != origFirst {
		t.Fatalf("FirstSeen should be immutable, was %d now %d", origFirst, row.FirstSeen)
	}
	if row.LeaseStart <= origStart {
		t.Fatalf("LeaseStart should have been reset forward, was %d now %d", origStart, row.LeaseStart)
	}
}

// A continuously-present device keeps its LeaseStart.
func TestUpdateLeases_KeepsLeaseStartWhilePresent(t *testing.T) {
	s := newTestDhcpService(t)
	now := CurrentTimestampMillis()
	origStart := now - 60*60*1000
	s.db.Create(&DhcpLeaseModel{
		MacAddress: "aa:bb:cc:dd:ee:ff",
		Server:     "dhcp-users",
		IPAddress:  "10.0.0.5",
		FirstSeen:  origStart,
		LeaseStart: origStart,
		LastSeen:   now - 30*1000, // seen 30s ago
	})
	s.prevSuccessTime = now - 60*1000 // last scrape 1 min ago; device seen since

	if err := s.updateLeases([]DhcpLeaseEntry{boundEntry()}); err != nil {
		t.Fatal(err)
	}
	row := getLease(t, s)
	if row.LeaseStart != origStart {
		t.Fatalf("LeaseStart should be unchanged, was %d now %d", origStart, row.LeaseStart)
	}
}

// On the very first scrape after a restart (prevSuccessTime == 0) we never reset
// LeaseStart, even for a row last seen long ago.
func TestUpdateLeases_NoResetOnRestart(t *testing.T) {
	s := newTestDhcpService(t)
	now := CurrentTimestampMillis()
	origStart := now - 60*60*1000
	s.db.Create(&DhcpLeaseModel{
		MacAddress: "aa:bb:cc:dd:ee:ff",
		Server:     "dhcp-users",
		IPAddress:  "10.0.0.5",
		FirstSeen:  origStart,
		LeaseStart: origStart,
		LastSeen:   now - 60*60*1000, // ancient
	})
	s.prevSuccessTime = 0 // fresh start

	if err := s.updateLeases([]DhcpLeaseEntry{boundEntry()}); err != nil {
		t.Fatal(err)
	}
	row := getLease(t, s)
	if row.LeaseStart != origStart {
		t.Fatalf("LeaseStart should not reset on restart, was %d now %d", origStart, row.LeaseStart)
	}
}
