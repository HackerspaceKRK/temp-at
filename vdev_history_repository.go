package main

import (
	"encoding/json"
	"log"
	"sync"

	"gorm.io/gorm"
)

// VirtualDeviceHistoryRepository stores virtual device state changes to the database.
type VirtualDeviceHistoryRepository struct {
	db        *gorm.DB
	deviceIDs map[string]uint // cache: device name -> DB ID
	mu        sync.Mutex
}

// NewVirtualDeviceHistoryRepository creates a new repository and registers as listener.
func NewVirtualDeviceHistoryRepository(db *gorm.DB, vdevManager *VdevManager) *VirtualDeviceHistoryRepository {
	repo := &VirtualDeviceHistoryRepository{
		db:        db,
		deviceIDs: make(map[string]uint),
	}

	// Register as listener for state changes
	vdevManager.OnVirtualDeviceUpdated = append(
		vdevManager.OnVirtualDeviceUpdated,
		repo.OnDeviceUpdated,
	)

	return repo
}

// OnDeviceUpdated is called when a virtual device state changes.
// It upserts the device record and inserts a new state entry.
// Note: camera_snapshot devices are excluded from history tracking.
func (r *VirtualDeviceHistoryRepository) OnDeviceUpdated(vdev *VirtualDevice) {
	// Skip camera_snapshot devices
	if vdev.Type == VdevTypeCameraSnapshot {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Get or create device ID
	deviceID, err := r.getOrCreateDeviceID(vdev.ID, string(vdev.Type))
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
		ID:              GenerateUUIDv7(),
		Timestamp:       CurrentTimestampMillis(),
		VirtualDeviceID: deviceID,
		State:           string(stateJSON),
	}

	if err := r.db.Create(&stateRecord).Error; err != nil {
		log.Printf("VirtualDeviceHistoryRepository: failed to insert state for %s: %v", vdev.ID, err)
		return
	}
}

// getOrCreateDeviceID returns the database ID for a device, creating it if necessary.
func (r *VirtualDeviceHistoryRepository) getOrCreateDeviceID(name string, deviceType string) (uint, error) {
	// Check cache first
	if id, ok := r.deviceIDs[name]; ok {
		return id, nil
	}

	// Use FirstOrCreate to upsert without "record not found" errors
	var device VirtualDeviceModel
	result := r.db.Where(VirtualDeviceModel{Name: name}).FirstOrCreate(&device, VirtualDeviceModel{
		Name: name,
		Type: deviceType,
	})

	if result.Error != nil {
		return 0, result.Error
	}

	r.deviceIDs[name] = device.ID
	return device.ID, nil
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
