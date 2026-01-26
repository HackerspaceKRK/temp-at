package main

import (
	"reflect"
	"sync"
)

// VdevType represents the type of a virtual device.
type VdevType string

const (
	VdevTypeRelay          VdevType = "relay"
	VdevTypeTemperature    VdevType = "temperature"
	VdevTypeHumidity       VdevType = "humidity"
	VdevTypePerson         VdevType = "person"
	VdevTypeCameraSnapshot VdevType = "camera_snapshot"
	VdevTypePowerUsage     VdevType = "power_usage"
	VdevTypeCo             VdevType = "co"
	VdevTypeGas            VdevType = "gas"
	VdevTypeContact        VdevType = "contact"
)

// VirtualDevice represents a single controllable/readable capability broken out
// from a physical device (e.g. multi-relay or multi-sensor).
// Moved from mqtt_adapter.go into this dedicated manager file.
type VirtualDevice struct {
	// ID is a string which uniquely identifies this virtual device.
	ID string `json:"id"`
	// Type: relay, temperature, humidity, person, etc.
	Type VdevType `json:"type"`
	// Current state of the given device (bool, float64, int, etc).
	State any `json:"state"`
	// MapperData stores any mapper-specific metadata.
	MapperData any `json:"mapper_data"`
	// Fresh indicates if the state is from a live update (true) or restored/initial (false).
	Fresh bool `json:"fresh"`
	// ProhibitControl indicates if this device cannot be controlled.
	ProhibitControl bool `json:"prohibit_control"`
}

// DeviceStateProvider defines the interface for retrieving persisted device state.
type DeviceStateProvider interface {
	GetLatestDeviceState(deviceID string) (any, error)
}

// VirtualDeviceUpdate represents a state change for a virtual device.
// Name must match an existing VirtualDevice ID for the update to apply.
type VirtualDeviceUpdate struct {
	Name  string `json:"name"`
	State any    `json:"state,omitempty"`
}

// VdevManager owns the in-memory collection of VirtualDevice objects and
// provides concurrency-safe mutation and snapshot retrieval APIs.
// Responsibility previously held inside MQTTAdapter has been extracted here.
type VdevManager struct {
	mu      sync.RWMutex
	devices []*VirtualDevice

	// OnVirtualDeviceUpdated callbacks are invoked for each device whose state changed.
	OnVirtualDeviceUpdated []func(vdev *VirtualDevice)

	stateProvider DeviceStateProvider
}

// NewVdevManager creates an empty manager instance.
func NewVdevManager() *VdevManager {
	return &VdevManager{}
}

// SetStateProvider configures the persistence provider.
func (m *VdevManager) SetStateProvider(p DeviceStateProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateProvider = p
}

// AddDevices adds newly discovered virtual devices whose IDs are not already present.
func (m *VdevManager) AddDevices(devs []*VirtualDevice) {
	if len(devs) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	existing := make(map[string]struct{}, len(m.devices))
	for _, d := range m.devices {
		existing[d.ID] = struct{}{}
	}

	for _, d := range devs {
		if d == nil || d.ID == "" {
			continue
		}
		if _, found := existing[d.ID]; found {
			continue
		}

		// Try to restore state if provider is available
		if m.stateProvider != nil {
			if persistedState, err := m.stateProvider.GetLatestDeviceState(d.ID); err == nil && persistedState != nil {
				d.State = persistedState
				// Fresh remains false (default) because this is a restored state
			}
		}

		m.devices = append(m.devices, d)
	}
}

// ApplyUpdates applies the provided updates to matching devices and returns
// the list of device IDs whose state actually changed.
// It also invokes the update callback for each changed device (if configured).
func (m *VdevManager) ApplyUpdates(updates []*VirtualDeviceUpdate) []string {
	if len(updates) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Index for O(1) lookup.
	index := make(map[string]*VirtualDevice, len(m.devices))
	for _, d := range m.devices {
		index[d.ID] = d
	}

	changed := make([]string, 0, len(updates))
	for _, upd := range updates {
		if upd == nil || upd.Name == "" {
			continue
		}
		if dev, ok := index[upd.Name]; ok {
			if shouldAssignState(dev.State, upd.State) {
				dev.State = upd.State
				dev.Fresh = true
				changed = append(changed, dev.ID)
			}
		}
	}

	// Fire callbacks outside the lock to avoid deadlocks.
	if len(changed) > 0 && len(m.OnVirtualDeviceUpdated) > 0 {
		// Collect updated devices for callbacks
		updatedDevices := make([]*VirtualDevice, 0, len(changed))
		for _, id := range changed {
			if dev, ok := index[id]; ok {
				clone := *dev
				updatedDevices = append(updatedDevices, &clone)
			}
		}
		callbacks := append([]func(vdev *VirtualDevice){}, m.OnVirtualDeviceUpdated...) // copy slice
		go func(devices []*VirtualDevice, cbs []func(vdev *VirtualDevice)) {
			for _, dev := range devices {
				for _, cb := range cbs {
					cb(dev)
				}
			}
		}(updatedDevices, callbacks)
	}

	return changed
}

// shouldAssignState returns true if newValue should replace oldValue.
// Comparable types are compared directly; non-comparable types always trigger assignment.
func shouldAssignState(oldValue, newValue any) bool {
	if oldValue == nil && newValue == nil {
		return false
	}
	if oldValue == nil || newValue == nil {
		return true
	}

	ov := reflect.ValueOf(oldValue)
	nv := reflect.ValueOf(newValue)

	// If either is non-comparable, we treat it as a changed value.
	if !ov.Type().Comparable() || !nv.Type().Comparable() {
		return true
	}
	return oldValue != newValue
}

// Devices returns a deep-ish (shallow copy of slice + struct copies) snapshot
// of all virtual devices. The returned slice and structs can be mutated
// by caller without affecting manager state.
func (m *VdevManager) Devices() []*VirtualDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make([]*VirtualDevice, len(m.devices))
	for i, dev := range m.devices {
		if dev == nil {
			continue
		}
		clone := *dev
		cp[i] = &clone
	}
	return cp
}
