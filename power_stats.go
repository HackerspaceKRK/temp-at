package main

import (
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
)

var powerStatsRanges = map[string]struct {
	duration time.Duration
	interval time.Duration
}{
	"48h": {48 * time.Hour, 5 * time.Minute},
	"7d":  {7 * 24 * time.Hour, 30 * time.Minute},
}

type powerGroup struct {
	series  NumericSeries
	sensors []string
	order   int
}

func handlePowerStats(c *fiber.Ctx) error {
	r, ok := powerStatsRanges[c.Query("range", "48h")]
	if !ok {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid range. Use '48h' or '7d'.")
	}

	cfg := MustLoadConfig()
	extra := make(map[string]bool, len(cfg.ExtraPowerMeters))
	for _, m := range cfg.ExtraPowerMeters {
		extra[m.ID] = true
	}

	var roomEntityIDs []string
	for _, room := range cfg.Rooms {
		for _, e := range room.Entities {
			if !extra[e.ID] {
				roomEntityIDs = append(roomEntityIDs, e.ID)
			}
		}
	}
	powerIDs, err := vdevHistoryRepo.FilterDeviceNamesByType(roomEntityIDs, VdevTypePowerUsage)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	isPower := make(map[string]bool, len(powerIDs))
	for _, id := range powerIDs {
		isPower[id] = true
	}

	var groups []powerGroup
	for _, room := range cfg.Rooms {
		g := powerGroup{
			series: NumericSeries{ID: room.ID, LocalizedName: room.LocalizedName, Color: room.Color},
			order:  room.PowerChartStackOrder,
		}
		for _, e := range room.Entities {
			if isPower[e.ID] {
				g.sensors = append(g.sensors, e.ID)
				isPower[e.ID] = false
			}
		}
		if len(g.sensors) > 0 {
			groups = append(groups, g)
		}
	}
	for _, m := range cfg.ExtraPowerMeters {
		groups = append(groups, powerGroup{
			series:  NumericSeries{ID: m.ID, LocalizedName: m.LocalizedName, Color: m.Color},
			sensors: []string{m.ID},
			order:   m.PowerChartStackOrder,
		})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].order < groups[j].order })

	var sensors []string
	for _, g := range groups {
		sensors = append(sensors, g.sensors...)
	}

	now := time.Now()
	timestamps, values, err := computeAveragedSeries(vdevHistoryRepo, sensors, now.Add(-r.duration), now, r.interval)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	resp := NumericSeriesResponse{Timestamps: timestamps, Series: []NumericSeries{}}
	if resp.Timestamps == nil {
		resp.Timestamps = []int64{}
	}
	for _, g := range groups {
		g.series.Values = sumSeries(len(timestamps), g.sensors, values)
		resp.Series = append(resp.Series, g.series)
	}
	return c.JSON(resp)
}

// sumSeries adds up the given series bucket by bucket; a bucket is nil only when all inputs are nil.
func sumSeries(n int, keys []string, values map[string][]*float64) []*float64 {
	result := make([]*float64, n)
	for _, k := range keys {
		for i, v := range values[k] {
			if v == nil {
				continue
			}
			if result[i] == nil {
				result[i] = new(float64)
			}
			*result[i] += *v
		}
	}
	return result
}
