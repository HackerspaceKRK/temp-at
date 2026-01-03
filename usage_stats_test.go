package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestProcessRoomHistory(t *testing.T) {
	// Setup
	bucketDuration := int64(60 * 60 * 1000) // 1 hour
	now := time.Now().UnixMilli()
	startTime := now - 2*bucketDuration

	dataPoints := []UsageHeatmapDataPoint{
		{StartsAt: startTime},
		{StartsAt: startTime + bucketDuration},
	}

	history := []VirtualDeviceStateModel{
		{
			Timestamp:     startTime + 10*60*1000, // 10 mins in
			VirtualDevice: VirtualDeviceModel{Name: "sensor1"},
			State:         "1",
		},
		{
			Timestamp:     startTime + 20*60*1000, // 20 mins in
			VirtualDevice: VirtualDeviceModel{Name: "sensor2"},
			State:         "2",
		},
		{
			Timestamp:     startTime + 30*60*1000, // 30 mins in
			VirtualDevice: VirtualDeviceModel{Name: "sensor1"},
			State:         "0",
		},
		{
			Timestamp:     startTime + bucketDuration + 10*60*1000, // 1h 10m in
			VirtualDevice: VirtualDeviceModel{Name: "sensor2"},
			State:         "0",
		},
	}

	// This is a global in main.go, but for the test we need to make sure we don't crash
	// In the real code it uses time.Now().UnixMilli() which IS now.

	processRoomHistory(history, dataPoints, bucketDuration, now)

	// Bucket 0:
	// 0-10m: 0 people
	// 10-20m: 1 person (sensor1)
	// 20-30m: 2 people (max of sensor1:1, sensor2:2)
	// 30m-1h: 2 people (max of sensor1:0, sensor2:2)

	// MaxPeople: 2
	// ManHours: (10m * 1 + 10m * 2 + 30m * 2) / 60m = (10 + 20 + 60) / 60 = 90 / 60 = 1.5
	// ActiveHours: (10m + 10m + 30m) / 60m = 50 / 60 = 0.8333...

	assert.Equal(t, 2, dataPoints[0].MaxPeople)
	assert.InDelta(t, 1.5, dataPoints[0].ManHours, 0.001)
	assert.InDelta(t, 50.0/60.0, dataPoints[0].ActiveHours, 0.001)

	// Bucket 1:
	// 0-10m (starts at 1h): 2 people (sensor2: 2)
	// 10m-1h: 0 people

	// MaxPeople: 2
	// ManHours: (10m * 2) / 60m = 20 / 60 = 0.333...
	// ActiveHours: 10m / 60m = 0.1666...

	assert.Equal(t, 2, dataPoints[1].MaxPeople)
	assert.InDelta(t, 20.0/60.0, dataPoints[1].ManHours, 0.001)
	assert.InDelta(t, 10.0/60.0, dataPoints[1].ActiveHours, 0.001)
}

func TestDistributeToBuckets(t *testing.T) {
	bucketDuration := int64(60 * 60 * 1000)
	startTime := int64(1000000000000)
	dataPoints := []UsageHeatmapDataPoint{
		{StartsAt: startTime},
		{StartsAt: startTime + bucketDuration},
	}

	// Span multiple buckets
	distributeToBuckets(startTime+30*60*1000, startTime+bucketDuration+30*60*1000, 2, dataPoints, bucketDuration)

	// Bucket 0: 30m overlap
	assert.Equal(t, 2, dataPoints[0].MaxPeople)
	assert.InDelta(t, 1.0, dataPoints[0].ManHours, 0.001)
	assert.InDelta(t, 0.5, dataPoints[0].ActiveHours, 0.001)

	// Bucket 1: 30m overlap
	assert.Equal(t, 2, dataPoints[1].MaxPeople)
	assert.InDelta(t, 1.0, dataPoints[1].ManHours, 0.001)
	assert.InDelta(t, 0.5, dataPoints[1].ActiveHours, 0.001)
}
