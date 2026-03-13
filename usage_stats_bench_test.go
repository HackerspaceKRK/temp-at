package main

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupBenchDB creates an in-memory SQLite DB with 60 days of sensor data for two rooms.
func setupBenchDB(b *testing.B) (*VirtualDeviceHistoryRepository, []RoomConfig) {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	if err := AutoMigrateModels(db); err != nil {
		b.Fatal(err)
	}

	// Create two presence sensors.
	sensor1 := VirtualDeviceModel{Name: "bench_sensor1", Type: "person"}
	sensor2 := VirtualDeviceModel{Name: "bench_sensor2", Type: "person"}
	db.Create(&sensor1)
	db.Create(&sensor2)

	now := time.Now()
	// Insert state changes for each of the past 60 days:
	// sensor1: arrives at 09:00, leaves at 18:00
	// sensor2: arrives at 10:00, leaves at 21:00
	for dayOffset := -60; dayOffset < 0; dayOffset++ {
		day := now.AddDate(0, 0, dayOffset)
		loc := day.Location()
		y, m, d := day.Date()

		type entry struct {
			deviceID uint
			hour     int
			count    string
		}
		entries := []entry{
			{sensor1.ID, 9, "1"},
			{sensor1.ID, 18, "0"},
			{sensor2.ID, 10, "2"},
			{sensor2.ID, 21, "0"},
		}
		for _, e := range entries {
			ts := time.Date(y, m, d, e.hour, 0, 0, 0, loc).UnixMilli()
			db.Create(&VirtualDeviceStateModel{
				ID:              GenerateUUIDv7(),
				Timestamp:       ts,
				VirtualDeviceID: e.deviceID,
				State:           e.count,
			})
		}
	}

	mgr := NewVdevManager()
	repo := NewVirtualDeviceHistoryRepository(db, mgr)

	rooms := []RoomConfig{
		{
			ID: "bench_room1",
			Entities: []EntityConfig{
				{ID: "bench_sensor1", Representation: "person"},
			},
		},
		{
			ID: "bench_room2",
			Entities: []EntityConfig{
				{ID: "bench_sensor2", Representation: "person"},
			},
		},
	}
	return repo, rooms
}

// BenchmarkUsageHeatmap60DaysColdCache benchmarks a 60-day daily query with no cache (old behaviour).
func BenchmarkUsageHeatmap60DaysColdCache(b *testing.B) {
	repo, rooms := setupBenchDB(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear cache to simulate cold start / pre-caching behaviour.
		repo.db.Where("1 = 1").Delete(&UsageStatsDayCache{})
		if _, err := computeUsageHeatmap(repo, rooms, "", "day", 60*24); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUsageHeatmap60DaysWarmCache benchmarks a 60-day daily query after the cache is populated.
func BenchmarkUsageHeatmap60DaysWarmCache(b *testing.B) {
	repo, rooms := setupBenchDB(b)
	// Prime the cache.
	if _, err := computeUsageHeatmap(repo, rooms, "", "day", 60*24); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := computeUsageHeatmap(repo, rooms, "", "day", 60*24); err != nil {
			b.Fatal(err)
		}
	}
}
