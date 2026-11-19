package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VdevHistoryTypeBlocklist defines device types whose history is not persisted
var VdevHistoryTypeBlocklist = map[VdevType]any{
	VdevTypeCameraSnapshot: true,
	VdevTypePrinter:        true,
}

const DeviceLastSeenAtUpdateInterval = 5 * time.Minute

// VirtualDeviceHistoryRepository stores virtual device state changes to the database.
type VirtualDeviceHistoryRepository struct {
	db             *gorm.DB
	dbDevicesCache map[string]*VirtualDeviceModel
	mu             sync.Mutex
}

// NewVirtualDeviceHistoryRepository creates a new repository and registers as listener.
func NewVirtualDeviceHistoryRepository(db *gorm.DB, vdevManager *VdevManager) *VirtualDeviceHistoryRepository {
	repo := &VirtualDeviceHistoryRepository{
		db:             db,
		dbDevicesCache: make(map[string]*VirtualDeviceModel),
	}

	// Register as listener for state changes
	vdevManager.OnVirtualDeviceUpdated = append(
		vdevManager.OnVirtualDeviceUpdated,
		repo.OnDeviceUpdated,
	)

	vdevManager.OnVirtualDeviceSeen = append(
		vdevManager.OnVirtualDeviceSeen,
		repo.OnDeviceSeen,
	)
	return repo
}

// OnDeviceUpdated is called when a virtual device state changes.
// It upserts the device record and inserts a new state entry.
func (r *VirtualDeviceHistoryRepository) OnDeviceUpdated(vdev *VirtualDevice) {
	if _, ok := VdevHistoryTypeBlocklist[vdev.Type]; ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create the device in the DB
	dbDevice, err := r.getOrCreateDbDevice(vdev.ID, string(vdev.Type))
	if err != nil {
		log.Printf("VirtualDeviceHistoryRepository: failed to get/create device %s: %v", vdev.ID, err)
		return
	}

	// Serialize state to JSON
	stateJSON, err := json.Marshal(vdev.State)
	if err != nil {
		log.Printf("VirtualDeviceHistoryRepository: failed to serialize state for %s: %v", vdev.ID, err)
		return
	}

	// Create state record
	stateRecord := VirtualDeviceStateModel{
		Timestamp:       CurrentTimestampMillis(),
		VirtualDeviceID: dbDevice.ID,
		State:           string(stateJSON),
	}

	if err := r.db.Create(&stateRecord).Error; err != nil {
		log.Printf("VirtualDeviceHistoryRepository: failed to insert state for %s: %v", vdev.ID, err)
		return
	}
}

// OnDeviceSeen sets LastSeenAt for the device
func (r *VirtualDeviceHistoryRepository) OnDeviceSeen(vdev *VirtualDevice) {
	if _, ok := VdevHistoryTypeBlocklist[vdev.Type]; ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	devModel, err := r.getOrCreateDbDevice(vdev.ID, string(vdev.Type))
	if err != nil {
		log.Printf("VirtualDeviceHistoryRepository: failed to get/create device %s: %v", vdev.ID, err)
		return
	}

	if devModel.LastSeenAt == nil || time.Since(*devModel.LastSeenAt) >= DeviceLastSeenAtUpdateInterval {
		now := time.Now()
		devModel.LastSeenAt = &now

		if err := r.db.Save(devModel).Error; err != nil {
			log.Printf("VirtualDeviceHistoryRepository: failed to update LastSeenAt for %s: %v", vdev.ID, err)
		}
	}
}

// getOrCreateDbDevice returns the database ID for a device, creating it if necessary.
func (r *VirtualDeviceHistoryRepository) getOrCreateDbDevice(name string, deviceType string) (*VirtualDeviceModel, error) {
	// Check cache first
	if dev, ok := r.dbDevicesCache[name]; ok {
		return dev, nil
	}

	// Use FirstOrCreate to upsert without "record not found" errors
	var device VirtualDeviceModel
	result := r.db.Where(VirtualDeviceModel{Name: name}).FirstOrCreate(&device, VirtualDeviceModel{
		Name: name,
		Type: deviceType,
	})

	if result.Error != nil {
		return nil, result.Error
	}

	r.dbDevicesCache[name] = &device
	return r.dbDevicesCache[name], nil
}

// GetLatestPersonDetectionTime returns the timestamp (in milliseconds) when a person was last detected
// for the given device. It finds the most recent transition from a positive count to zero.
// Returns nil if the person is still detected (current state is positive) or if no history exists.
func (r *VirtualDeviceHistoryRepository) GetLatestPersonDetectionTime(deviceName string) (*int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get device ID
	var device VirtualDeviceModel
	if err := r.db.Where("name = ?", deviceName).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No history for this device
		}
		return nil, err
	}

	// Query recent state history (last 100 records should be sufficient)
	var states []VirtualDeviceStateModel
	if err := r.db.Where("virtual_device_id = ?", device.ID).
		Order("timestamp DESC").
		Limit(100).
		Find(&states).Error; err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return nil, nil // No state history
	}

	// Parse states and find transition from positive to zero
	// States are ordered DESC by timestamp, so we iterate from most recent to oldest
	for i := 0; i < len(states); i++ {
		var currentCount int
		if err := json.Unmarshal([]byte(states[i].State), &currentCount); err != nil {
			continue // Skip malformed states
		}

		// If current (most recent) state is positive, person is still present
		if i == 0 && currentCount > 0 {
			return nil, nil
		}

		// Look for transition: current state is 0, previous state (older) was positive
		if currentCount == 0 && i+1 < len(states) {
			var previousCount int
			if err := json.Unmarshal([]byte(states[i+1].State), &previousCount); err != nil {
				continue
			}

			if previousCount > 0 {
				// Found transition from positive to zero
				// Return the timestamp of the zero state (when person left)
				return &states[i].Timestamp, nil
			}
		}
	}

	return nil, nil // No transition found
}

// GetLatestDeviceState returns the most recent state for the given device ID.
// Returns nil, nil if no state is found.
func (r *VirtualDeviceHistoryRepository) GetLatestDeviceState(deviceID string) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Get database ID for the device string ID
	var device VirtualDeviceModel
	if err := r.db.Where("name = ?", deviceID).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// 2. Get the latest state record
	var latestState VirtualDeviceStateModel
	err := r.db.Where("virtual_device_id = ?", device.ID).
		Order("timestamp DESC").
		First(&latestState).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	// 3. Unmarshal the state
	var state any
	if err := json.Unmarshal([]byte(latestState.State), &state); err != nil {
		return nil, err
	}

	return state, nil
}

// GetDeviceHistory returns the state history for a device within a specific duration.
func (r *VirtualDeviceHistoryRepository) GetDeviceHistory(deviceName string, durationMs int64) ([]VirtualDeviceStateModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Get database ID for the device string ID
	var device VirtualDeviceModel
	if err := r.db.Where("name = ?", deviceName).First(&device).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No device found, so no history
		}
		return nil, err
	}

	// 2. Calculate cutoff timestamp
	cutoff := CurrentTimestampMillis() - durationMs

	// 3. Query history
	var history []VirtualDeviceStateModel
	err := r.db.Where("virtual_device_id = ? AND timestamp >= ?", device.ID, cutoff).
		Order("timestamp ASC"). // Oldest first usually makes sense for charts, but requester didn't specify. ASC is standard for time series.
		Find(&history).Error

	return history, err
}

// GetDevicesHistoryInRange returns the state history for multiple devices within an absolute time range.
// History is returned sorted by timestamp ascending.
func (r *VirtualDeviceHistoryRepository) GetDevicesHistoryInRange(deviceNames []string, fromMs, toMs int64) ([]VirtualDeviceStateModel, error) {
	if len(deviceNames) == 0 {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	var devices []VirtualDeviceModel
	if err := r.db.Where("name IN ?", deviceNames).Find(&devices).Error; err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, nil
	}

	deviceIDs := make([]uint, len(devices))
	for i, d := range devices {
		deviceIDs[i] = d.ID
	}

	var history []VirtualDeviceStateModel
	err := r.db.Preload("VirtualDevice").
		Where("virtual_device_id IN ? AND timestamp >= ? AND timestamp < ?", deviceIDs, fromMs, toMs).
		Order("timestamp ASC").
		Find(&history).Error
	return history, err
}

// GetDayCaches returns cached daily stats for the given roomID and date strings ("2006-01-02").
// The returned map is keyed by date string.
func (r *VirtualDeviceHistoryRepository) GetDayCaches(roomID string, dates []string) (map[string]*UsageStatsDayCache, error) {
	if len(dates) == 0 {
		return map[string]*UsageStatsDayCache{}, nil
	}

	var rows []UsageStatsDayCache
	if err := r.db.Where("room_id = ? AND date IN ?", roomID, dates).Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*UsageStatsDayCache, len(rows))
	for i := range rows {
		result[rows[i].Date] = &rows[i]
	}
	return result, nil
}

// UpsertDayCache inserts or updates a daily cache entry identified by (RoomID, Date).
func (r *VirtualDeviceHistoryRepository) UpsertDayCache(cache *UsageStatsDayCache) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "room_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{"max_people", "man_hours", "active_hours", "hourly_data"}),
	}).Create(cache).Error
}

// GetNumericSeriesCaches returns cached day series keyed by series key, then date.
func (r *VirtualDeviceHistoryRepository) GetNumericSeriesCaches(keys []string, intervalMinutes int, dates []string) (map[string]map[string]*NumericSeriesDayCache, error) {
	result := make(map[string]map[string]*NumericSeriesDayCache)
	if len(keys) == 0 || len(dates) == 0 {
		return result, nil
	}

	var rows []NumericSeriesDayCache
	if err := r.db.Where("series_key IN ? AND interval_minutes = ? AND date IN ?", keys, intervalMinutes, dates).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		if result[rows[i].SeriesKey] == nil {
			result[rows[i].SeriesKey] = make(map[string]*NumericSeriesDayCache)
		}
		result[rows[i].SeriesKey][rows[i].Date] = &rows[i]
	}
	return result, nil
}

func (r *VirtualDeviceHistoryRepository) UpsertNumericSeriesCache(cache *NumericSeriesDayCache) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "series_key"}, {Name: "date"}, {Name: "interval_minutes"}},
		DoUpdates: clause.AssignmentColumns([]string{"data"}),
	}).Create(cache).Error
}

// GetLastStatesBefore returns, per device name, the latest state recorded before ts.
func (r *VirtualDeviceHistoryRepository) GetLastStatesBefore(deviceNames []string, ts int64) (map[string]VirtualDeviceStateModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var devices []VirtualDeviceModel
	if err := r.db.Where("name IN ?", deviceNames).Find(&devices).Error; err != nil {
		return nil, err
	}

	result := make(map[string]VirtualDeviceStateModel, len(devices))
	for _, d := range devices {
		var rows []VirtualDeviceStateModel
		err := r.db.Where("virtual_device_id = ? AND timestamp < ?", d.ID, ts).
			Order("timestamp DESC").Limit(1).Find(&rows).Error
		if err != nil {
			return nil, err
		}
		if len(rows) > 0 {
			rows[0].VirtualDevice = d
			result[d.Name] = rows[0]
		}
	}
	return result, nil
}

// FilterDeviceNamesByType returns the subset of names whose recorded device type is t.
func (r *VirtualDeviceHistoryRepository) FilterDeviceNamesByType(names []string, t VdevType) ([]string, error) {
	var result []string
	if len(names) == 0 {
		return result, nil
	}
	err := r.db.Model(&VirtualDeviceModel{}).Where("name IN ? AND type = ?", names, string(t)).Pluck("name", &result).Error
	return result, err
}

// GetDevicesHistory returns the state history for multiple devices within a specific duration.
func (r *VirtualDeviceHistoryRepository) GetDevicesHistory(deviceNames []string, durationMs int64) ([]VirtualDeviceStateModel, error) {
	if len(deviceNames) == 0 {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Get database IDs for the device string IDs
	var devices []VirtualDeviceModel
	if err := r.db.Where("name IN ?", deviceNames).Find(&devices).Error; err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, nil
	}

	deviceIDs := make([]uint, len(devices))
	for i, d := range devices {
		deviceIDs[i] = d.ID
	}

	// 2. Calculate cutoff timestamp
	cutoff := CurrentTimestampMillis() - durationMs

	// 3. Query history
	var history []VirtualDeviceStateModel
	err := r.db.Preload("VirtualDevice").
		Where("virtual_device_id IN ? AND timestamp >= ?", deviceIDs, cutoff).
		Order("timestamp ASC").
		Find(&history).Error

	return history, err
}
