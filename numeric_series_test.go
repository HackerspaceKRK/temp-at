package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAverageBuckets(t *testing.T) {
	starts := []int64{0, 100, 200, 300}
	samples := []numericSample{{ts: 50, value: 10}, {ts: 150, value: 30}}

	values := averageBuckets(nil, samples, starts, 100, 250)

	require.Len(t, values, 4)
	assert.InDelta(t, 10, *values[0], 1e-9)
	assert.InDelta(t, 20, *values[1], 1e-9)
	assert.InDelta(t, 30, *values[2], 1e-9)
	assert.Nil(t, values[3])
}

func TestAverageBucketsSeed(t *testing.T) {
	seed := &numericSample{ts: -500, value: 4}
	values := averageBuckets(seed, []numericSample{{ts: 75, value: 8}}, []int64{0}, 100, 1000)
	assert.InDelta(t, 5, *values[0], 1e-9)
}

func TestComputeAveragedSeriesCaches(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, AutoMigrateModels(db))
	repo := &VirtualDeviceHistoryRepository{db: db, deviceIDs: map[string]uint{}}

	dev := VirtualDeviceModel{Name: "power1", Type: string(VdevTypePowerUsage)}
	require.NoError(t, db.Create(&dev).Error)

	now := time.Now()
	yesterday := startOfDay(now).AddDate(0, 0, -1)
	for _, s := range []struct {
		at    time.Time
		state string
	}{
		{yesterday.Add(-time.Hour), "100"},
		{yesterday.Add(12 * time.Hour), "200"},
	} {
		require.NoError(t, db.Create(&VirtualDeviceStateModel{
			ID: GenerateUUIDv7(), Timestamp: s.at.UnixMilli(), VirtualDeviceID: dev.ID, State: s.state,
		}).Error)
	}

	from := yesterday
	timestamps, series, err := computeAveragedSeries(repo, []string{"power1"}, from, now, 30*time.Minute)
	require.NoError(t, err)
	require.Equal(t, len(timestamps), len(series["power1"]))
	assert.InDelta(t, 100, *series["power1"][0], 1e-9)
	assert.InDelta(t, 200, *series["power1"][len(timestamps)-1], 1e-9)

	var count int64
	db.Model(&NumericSeriesDayCache{}).Count(&count)
	assert.Equal(t, int64(1), count)

	timestamps2, series2, err := computeAveragedSeries(repo, []string{"power1"}, from, now, 30*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, timestamps, timestamps2)
	assert.Equal(t, series, series2)
}
