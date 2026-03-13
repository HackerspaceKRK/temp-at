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

	resp, err := computeUsageHeatmap(vdevHistoryRepo, rooms, roomId, resolution, durationHours)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.JSON(resp)
}

// computedDayData holds the pre-computed results for a single calendar day.
type computedDayData struct {
	daily  UsageHeatmapDataPoint
	hourly []UsageHeatmapDataPoint // always 24 entries (one per hour, 0–23)
}

// computeUsageHeatmap is the core logic, extracted for testability.
// cacheKey is the roomId (or "" for all rooms).
func computeUsageHeatmap(repo *VirtualDeviceHistoryRepository, rooms []RoomConfig, cacheKey, resolution string, durationHours int) (*UsageHeatmapResponse, error) {
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
		return &UsageHeatmapResponse{DataPoints: []UsageHeatmapDataPoint{}}, nil
	}

	now := time.Now()
	durationMs := int64(durationHours) * 60 * 60 * 1000

	// Round start time to resolution boundary.
	t := time.UnixMilli(now.UnixMilli() - durationMs)
	var startTime time.Time
	var bucketDuration time.Duration
	if resolution == "day" {
		startTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		bucketDuration = 24 * time.Hour
	} else {
		startTime = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
		bucketDuration = time.Hour
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Enumerate all bucket start times.
	var bucketTimes []time.Time
	for bt := startTime; bt.Before(now); bt = bt.Add(bucketDuration) {
		bucketTimes = append(bucketTimes, bt)
	}
	if len(bucketTimes) == 0 {
		return &UsageHeatmapResponse{DataPoints: []UsageHeatmapDataPoint{}}, nil
	}

	// --- Cache lookup for complete days ---

	// Collect the distinct calendar days covered, split by complete vs today.
	completeDaySet := make(map[string]time.Time) // dateStr -> day start (only past days)
	includeToday := false
	for _, bt := range bucketTimes {
		dayStart := time.Date(bt.Year(), bt.Month(), bt.Day(), 0, 0, 0, 0, bt.Location())
		if dayStart.Before(todayStart) {
			completeDaySet[dayStart.Format("2006-01-02")] = dayStart
		} else {
			includeToday = true
		}
	}

	completeDates := make([]string, 0, len(completeDaySet))
	for dateStr := range completeDaySet {
		completeDates = append(completeDates, dateStr)
	}

	caches, err := repo.GetDayCaches(cacheKey, completeDates)
	if err != nil {
		return nil, err
	}

	// Days that need DB computation: complete days not in cache, plus today.
	daysToCompute := make(map[string]time.Time)
	for dateStr, dayStart := range completeDaySet {
		if caches[dateStr] == nil {
			daysToCompute[dateStr] = dayStart
		}
	}
	todayDateStr := todayStart.Format("2006-01-02")
	if includeToday {
		daysToCompute[todayDateStr] = todayStart
	}

	// --- DB query for uncached days ---

	computedResults := make(map[string]computedDayData)
	if len(daysToCompute) > 0 {
		var minDay, maxDay time.Time
		first := true
		for _, d := range daysToCompute {
			if first || d.Before(minDay) {
				minDay = d
			}
			if first || d.After(maxDay) {
				maxDay = d
			}
			first = false
		}

		// Query from 2 hours before the earliest day to capture sensors active around midnight.
		queryFrom := minDay.Add(-2 * time.Hour)
		queryTo := maxDay.Add(24 * time.Hour)
		if queryTo.After(now) {
			queryTo = now
		}

		history, err := repo.GetDevicesHistoryInRange(sensorNames, queryFrom.UnixMilli(), queryTo.UnixMilli())
		if err != nil {
			return nil, err
		}

		for dateStr, dayStart := range daysToCompute {
			dayEnd := dayStart.Add(24 * time.Hour)
			if dayEnd.After(now) {
				dayEnd = now
			}

			// Filter sorted history to [dayStart-2h, dayEnd) using binary search.
			lookbackMs := dayStart.Add(-2 * time.Hour).UnixMilli()
			dayHistory := filterHistoryInRange(history, lookbackMs, dayEnd.UnixMilli())

			daily, hourly := computeDayBuckets(dayHistory, roomToSensors, dayStart, dayEnd)
			computedResults[dateStr] = computedDayData{daily: daily, hourly: hourly}

			// Upsert cache only for complete (past) days.
			if dayStart.Before(todayStart) {
				hourlyJSON, _ := json.Marshal(hourly)
				_ = repo.UpsertDayCache(&UsageStatsDayCache{
					RoomID:      cacheKey,
					Date:        dateStr,
					MaxPeople:   daily.MaxPeople,
					ManHours:    daily.ManHours,
					ActiveHours: daily.ActiveHours,
					HourlyData:  string(hourlyJSON),
				})
			}
		}
	}

	// --- Assemble response ---

	dataPoints := make([]UsageHeatmapDataPoint, 0, len(bucketTimes))
	for _, bt := range bucketTimes {
		dateStr := time.Date(bt.Year(), bt.Month(), bt.Day(), 0, 0, 0, 0, bt.Location()).Format("2006-01-02")
		dp := UsageHeatmapDataPoint{StartsAt: bt.UnixMilli()}

		if resolution == "day" {
			if cached, ok := caches[dateStr]; ok {
				dp.MaxPeople = cached.MaxPeople
				dp.ManHours = cached.ManHours
				dp.ActiveHours = cached.ActiveHours
			} else if computed, ok := computedResults[dateStr]; ok {
				dp.MaxPeople = computed.daily.MaxPeople
				dp.ManHours = computed.daily.ManHours
				dp.ActiveHours = computed.daily.ActiveHours
			}
		} else { // hourly
			if cached, ok := caches[dateStr]; ok {
				var hourlyData []UsageHeatmapDataPoint
				if err := json.Unmarshal([]byte(cached.HourlyData), &hourlyData); err == nil {
					for _, h := range hourlyData {
						if h.StartsAt == bt.UnixMilli() {
							dp = h
							break
						}
					}
				}
			} else if computed, ok := computedResults[dateStr]; ok {
				for _, h := range computed.hourly {
					if h.StartsAt == bt.UnixMilli() {
						dp = h
						break
					}
				}
			}
		}

		dataPoints = append(dataPoints, dp)
	}

	return &UsageHeatmapResponse{DataPoints: dataPoints}, nil
}

// filterHistoryInRange returns the slice of history records with timestamp in [fromMs, toMs).
// Assumes history is sorted by timestamp ascending.
func filterHistoryInRange(history []VirtualDeviceStateModel, fromMs, toMs int64) []VirtualDeviceStateModel {
	lo := sort.Search(len(history), func(i int) bool { return history[i].Timestamp >= fromMs })
	hi := sort.Search(len(history), func(i int) bool { return history[i].Timestamp >= toMs })
	return history[lo:hi]
}

// computeDayBuckets computes hourly stats for a single calendar day and derives the daily aggregate.
// history must be pre-filtered to include events from dayStart-2h onwards.
// dayEnd is the exclusive end (either dayStart+24h or now for today).
func computeDayBuckets(history []VirtualDeviceStateModel, roomToSensors map[string][]string, dayStart, dayEnd time.Time) (UsageHeatmapDataPoint, []UsageHeatmapDataPoint) {
	dayStartMs := dayStart.UnixMilli()
	dayEndMs := dayEnd.UnixMilli()
	hourMs := int64(time.Hour / time.Millisecond)

	// 24 hourly buckets for the day.
	hourlyPoints := make([]UsageHeatmapDataPoint, 24)
	for i := range hourlyPoints {
		hourlyPoints[i].StartsAt = dayStartMs + int64(i)*hourMs
	}

	// Group history records by room.
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

	// Accumulate each room's contribution into the shared hourly buckets.
	for _, events := range roomHistory {
		processRoomHistory(events, hourlyPoints, hourMs, dayEndMs)
	}

	// Derive daily aggregate from hourly buckets.
	daily := UsageHeatmapDataPoint{StartsAt: dayStartMs}
	for _, h := range hourlyPoints {
		if h.MaxPeople > daily.MaxPeople {
			daily.MaxPeople = h.MaxPeople
		}
		daily.ManHours += h.ManHours
		daily.ActiveHours += h.ActiveHours
	}

	return daily, hourlyPoints
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
