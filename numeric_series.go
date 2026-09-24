package main

import (
	"encoding/json"
	"sort"
	"time"
)

type numericSample struct {
	ts    int64
	value float64
}

type NumericSeries struct {
	ID            string          `json:"id"`
	LocalizedName LocalizedString `json:"localized_name"`
	Color         string          `json:"color,omitempty"`
	Values        []*float64      `json:"values"`
}

type NumericSeriesResponse struct {
	Timestamps []int64         `json:"timestamps"`
	Series     []NumericSeries `json:"series"`
}

func parseNumericState(state string) (float64, bool) {
	var v float64
	if err := json.Unmarshal([]byte(state), &v); err != nil {
		return 0, false
	}
	return v, true
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func dayBucketStarts(dayStart time.Time, interval time.Duration) []int64 {
	end := startOfDay(dayStart.Add(36 * time.Hour)).UnixMilli()
	step := interval.Milliseconds()
	var starts []int64
	for ts := dayStart.UnixMilli(); ts < end; ts += step {
		starts = append(starts, ts)
	}
	return starts
}

// averageBuckets computes the time-weighted average of a step function per bucket.
// Buckets start at the given timestamps, span interval ms and are clipped to until.
func averageBuckets(seed *numericSample, samples []numericSample, bucketStarts []int64, interval, until int64) []*float64 {
	result := make([]*float64, len(bucketStarts))
	current := seed
	i := 0
	for b, bs := range bucketStarts {
		be := min(bs+interval, until)
		if bs >= be {
			continue
		}
		var sum, covered float64
		pos := bs
		for pos < be {
			for i < len(samples) && samples[i].ts <= pos {
				current = &samples[i]
				i++
			}
			next := be
			if i < len(samples) && samples[i].ts < be {
				next = samples[i].ts
			}
			if current != nil {
				d := float64(next - pos)
				sum += current.value * d
				covered += d
			}
			pos = next
		}
		if covered > 0 {
			avg := sum / covered
			result[b] = &avg
		}
	}
	return result
}

// computeAveragedSeries returns interval averages for each numeric device over [from, to).
// Complete past days are served from and stored in NumericSeriesDayCache.
func computeAveragedSeries(repo *VirtualDeviceHistoryRepository, deviceNames []string, from, to time.Time, interval time.Duration) ([]int64, map[string][]*float64, error) {
	intervalMinutes := int(interval / time.Minute)
	todayStart := startOfDay(to)

	var days []time.Time
	for d := startOfDay(from); d.Before(to); d = startOfDay(d.Add(36 * time.Hour)) {
		days = append(days, d)
	}

	var completeDates []string
	for _, d := range days {
		if d.Before(todayStart) {
			completeDates = append(completeDates, d.Format("2006-01-02"))
		}
	}
	caches, err := repo.GetNumericSeriesCaches(deviceNames, intervalMinutes, completeDates)
	if err != nil {
		return nil, nil, err
	}

	dayData := make(map[string]map[string][]*float64, len(deviceNames))
	for _, name := range deviceNames {
		dayData[name] = make(map[string][]*float64)
		for date, c := range caches[name] {
			var values []*float64
			if json.Unmarshal([]byte(c.Data), &values) == nil {
				dayData[name][date] = values
			}
		}
	}

	var missing []time.Time
	for _, d := range days {
		date := d.Format("2006-01-02")
		for _, name := range deviceNames {
			if _, ok := dayData[name][date]; !ok {
				missing = append(missing, d)
				break
			}
		}
	}

	if len(missing) > 0 {
		queryFrom := missing[0]
		queryTo := min(startOfDay(missing[len(missing)-1].Add(36*time.Hour)).UnixMilli(), to.UnixMilli())

		seeds, err := repo.GetLastStatesBefore(deviceNames, queryFrom.UnixMilli())
		if err != nil {
			return nil, nil, err
		}
		history, err := repo.GetDevicesHistoryInRange(deviceNames, queryFrom.UnixMilli(), queryTo)
		if err != nil {
			return nil, nil, err
		}

		samples := make(map[string][]numericSample)
		for _, name := range deviceNames {
			if s, ok := seeds[name]; ok {
				if v, ok := parseNumericState(s.State); ok {
					samples[name] = append(samples[name], numericSample{ts: s.Timestamp, value: v})
				}
			}
		}
		for _, h := range history {
			if v, ok := parseNumericState(h.State); ok {
				samples[h.VirtualDevice.Name] = append(samples[h.VirtualDevice.Name], numericSample{ts: h.Timestamp, value: v})
			}
		}

		for _, d := range missing {
			date := d.Format("2006-01-02")
			starts := dayBucketStarts(d, interval)
			dayStartMs := d.UnixMilli()
			dayEndMs := starts[len(starts)-1] + interval.Milliseconds()
			for _, name := range deviceNames {
				if _, ok := dayData[name][date]; ok {
					continue
				}
				s := samples[name]
				lo := sort.Search(len(s), func(i int) bool { return s[i].ts >= dayStartMs })
				hi := sort.Search(len(s), func(i int) bool { return s[i].ts >= dayEndMs })
				var seed *numericSample
				if lo > 0 {
					seed = &s[lo-1]
				}
				values := averageBuckets(seed, s[lo:hi], starts, interval.Milliseconds(), to.UnixMilli())
				dayData[name][date] = values

				if d.Before(todayStart) {
					data, _ := json.Marshal(values)
					_ = repo.UpsertNumericSeriesCache(&NumericSeriesDayCache{
						SeriesKey:       name,
						Date:            date,
						IntervalMinutes: intervalMinutes,
						Data:            string(data),
					})
				}
			}
		}
	}

	var timestamps []int64
	series := make(map[string][]*float64, len(deviceNames))
	fromMs, toMs := from.UnixMilli(), to.UnixMilli()
	for _, d := range days {
		date := d.Format("2006-01-02")
		for i, ts := range dayBucketStarts(d, interval) {
			if ts+interval.Milliseconds() <= fromMs || ts >= toMs {
				continue
			}
			timestamps = append(timestamps, ts)
			for _, name := range deviceNames {
				var v *float64
				if values := dayData[name][date]; i < len(values) {
					v = values[i]
				}
				series[name] = append(series[name], v)
			}
		}
	}
	return timestamps, series, nil
}
