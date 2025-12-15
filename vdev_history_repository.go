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
