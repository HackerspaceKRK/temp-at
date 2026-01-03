package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
)

type UsageHeatmapDataPoint struct {
	StartsAt    int64   `json:"startsAt"`
	MaxPeople   int     `json:"maxPeople"`
	ManHours    float64 `json:"manHours"`
	ActiveHours float64 `json:"activeHours"`
}

type UsageHeatmapResponse struct {
	DataPoints []UsageHeatmapDataPoint `json:"dataPoints"`
}

const (
	MaxDailyDurationHours      = 60 * 24
	MaxHourlyDurationHours     = 14 * 24
	DefaultDailyDurationHours  = 30 * 24
	DefaultHourlyDurationHours = 7 * 24
)

func handleUsageHeatmap(c *fiber.Ctx) error {
	roomId := c.Query("roomId")
	resolution := c.Query("resolution", "day")
	durationStr := c.Query("duration")

	var durationHours int
	if resolution == "day" {
		if durationStr == "" {
			durationHours = DefaultDailyDurationHours
		} else {
			fmt.Sscanf(durationStr, "%d", &durationHours)
			durationHours *= 24
		}
		if durationHours > MaxDailyDurationHours {
			durationHours = MaxDailyDurationHours
		}
	} else if resolution == "hour" {
		if durationStr == "" {
			durationHours = DefaultHourlyDurationHours
		} else {
			fmt.Sscanf(durationStr, "%d", &durationHours)
		}
		if durationHours > MaxHourlyDurationHours {
			durationHours = MaxHourlyDurationHours
		}
	} else {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid resolution. Use 'day' or 'hour'.")
	}

	cfg := MustLoadConfig()
	var rooms []RoomConfig
	if roomId != "" {
		for _, r := range cfg.Rooms {
			if r.ID == roomId {
				rooms = append(rooms, r)
				break
			}
		}
		if len(rooms) == 0 {
			return c.Status(fiber.StatusNotFound).SendString("Room not found")
		}
	} else {
		rooms = cfg.Rooms
	}

	var sensorNames []string
	roomToSensors := make(map[string][]string)
	for _, r := range rooms {
		for _, e := range r.Entities {
			if e.Representation == "presence" || e.Representation == "person" {
				sensorNames = append(sensorNames, e.ID)
				roomToSensors[r.ID] = append(roomToSensors[r.ID], e.ID)
			}
		}
	}

	if len(sensorNames) == 0 {
		// Return empty response if no presence sensors are configured
		return c.JSON(UsageHeatmapResponse{DataPoints: []UsageHeatmapDataPoint{}})
	}

	durationMs := int64(durationHours) * 60 * 60 * 1000
	history, err := vdevHistoryRepo.GetDevicesHistory(sensorNames, durationMs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	// Group history by room
	roomHistory := make(map[string][]VirtualDeviceStateModel)
	for _, h := range history {
		for rid, sensors := range roomToSensors {
			for _, sname := range sensors {
				if h.VirtualDevice.Name == sname {
					roomHistory[rid] = append(roomHistory[rid], h)
					break
				}
			}
		}
	}

	now := time.Now().UnixMilli()
	startTime := now - durationMs

	// Round start time to resolution boundary
	t := time.UnixMilli(startTime)
	if resolution == "day" {
		startTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).UnixMilli()
	} else {
		startTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location()).UnixMilli()
	}

	var bucketDuration int64
	if resolution == "day" {
		bucketDuration = 24 * 60 * 60 * 1000
	} else {
		bucketDuration = 60 * 60 * 1000
	}

	numBuckets := int((now - startTime) / bucketDuration)
	if (now-startTime)%bucketDuration != 0 {
		numBuckets++
	}

	dataPoints := make([]UsageHeatmapDataPoint, numBuckets)
	for i := 0; i < numBuckets; i++ {
		dataPoints[i] = UsageHeatmapDataPoint{
			StartsAt: startTime + int64(i)*bucketDuration,
		}
	}

	for _, events := range roomHistory {
		processRoomHistory(events, dataPoints, bucketDuration, now)
	}

	return c.JSON(UsageHeatmapResponse{DataPoints: dataPoints})
}

type event struct {
	timestamp int64
	sensor    string
	count     int
}

func processRoomHistory(history []VirtualDeviceStateModel, dataPoints []UsageHeatmapDataPoint, bucketDuration int64, now int64) {
	if len(history) == 0 {
		return
	}

	// Parse events and group by sensor to know initial states if needed
	// But it's easier to just sort all events and maintain current state per sensor
	var events []event
	sensorStates := make(map[string]int)

	for _, h := range history {
		var count int
		err := json.Unmarshal([]byte(h.State), &count)
		if err != nil {
			// Try bool if int fails (presence sensors sometimes report bool)
			var b bool
			if json.Unmarshal([]byte(h.State), &b) == nil {
				if b {
					count = 1
				} else {
					count = 0
				}
			} else {
				continue
			}
		}
		events = append(events, event{
			timestamp: h.Timestamp,
			sensor:    h.VirtualDevice.Name,
			count:     count,
		})
		sensorStates[h.VirtualDevice.Name] = 0 // Initialize
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].timestamp < events[j].timestamp
	})

	if len(events) == 0 {
		return
	}

	currentRoomMax := func() int {
		max := 0
		for _, c := range sensorStates {
			if c > max {
				max = c
			}
		}
		return max
	}

	// Add an end event for "now" to finish calculation
	events = append(events, event{timestamp: now})

	lastTimestamp := events[0].timestamp
	for _, e := range events {
		duration := e.timestamp - lastTimestamp
		if duration > 0 {
			occupancy := currentRoomMax()

			// Distribute this duration across buckets
			distributeToBuckets(lastTimestamp, e.timestamp, occupancy, dataPoints, bucketDuration)
		}

		if e.sensor != "" {
			sensorStates[e.sensor] = e.count
		}
		lastTimestamp = e.timestamp
	}
}

func distributeToBuckets(start, end int64, occupancy int, dataPoints []UsageHeatmapDataPoint, bucketDuration int64) {
	if occupancy < 0 {
		occupancy = 0
	}

	for i := range dataPoints {
		bStart := dataPoints[i].StartsAt
		bEnd := bStart + bucketDuration

		// Overlap
		oStart := start
		if bStart > oStart {
			oStart = bStart
		}
		oEnd := end
		if bEnd < oEnd {
			oEnd = bEnd
		}

		if oStart < oEnd {
			overlapMs := oEnd - oStart
			overlapHours := float64(overlapMs) / (60 * 60 * 1000)

			if occupancy > dataPoints[i].MaxPeople {
				dataPoints[i].MaxPeople = occupancy
			}
			dataPoints[i].ManHours += float64(occupancy) * overlapHours
			if occupancy > 0 {
				dataPoints[i].ActiveHours += overlapHours
			}
		}
	}
}
