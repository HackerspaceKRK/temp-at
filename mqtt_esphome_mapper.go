package main

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ESPHomeMapperData stores metadata for ESPHome virtual devices.
type ESPHomeMapperData struct {
	StateTopic string `json:"state_topic"`
	UniqueID   string `json:"unique_id"`
}

// ESPHomeConfig represents the JSON configuration received from Home Assistant discovery.
type ESPHomeConfig struct {
	DeviceClass       string `json:"dev_cla"`
	UnitOfMeasurement string `json:"unit_of_meas"`
	StateClass        string `json:"stat_cla"`
	Name              string `json:"name"`
	StateTopic        string `json:"stat_t"`
	AvailabilityTopic string `json:"avty_t"`
	UniqueID          string `json:"uniq_id"`
}

// ESPHomeMapper implements MQTTMapper for ESPHome devices using Home Assistant discovery topics.
type ESPHomeMapper struct {
	mu sync.RWMutex
	// devicesByStateTopic maps state topics to virtual devices.
	devicesByStateTopic map[string][]*VirtualDevice
}

// NewESPHomeMapper creates a new ESPHome mapper.
func NewESPHomeMapper() *ESPHomeMapper {
	return &ESPHomeMapper{
		devicesByStateTopic: make(map[string][]*VirtualDevice),
	}
}

// SubscriptionTopics returns topics needed for ESPHome discovery and updates.
func (m *ESPHomeMapper) SubscriptionTopics() []string {
	return []string{
		"homeassistant/sensor/+/+/config",
		"+/sensor/+/state",
	}
}

// DiscoverDevicesFromMessage parses config payloads and builds virtual devices.
func (m *ESPHomeMapper) DiscoverDevicesFromMessage(topic string, payload []byte) ([]*VirtualDevice, error) {
	// Only handle homeassistant/sensor/+/+/config
	if !strings.HasPrefix(topic, "homeassistant/sensor/") || !strings.HasSuffix(topic, "/config") {
		return nil, nil
	}

	if len(payload) == 0 {
		return nil, nil
	}

	var config ESPHomeConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, err
	}

	if config.DeviceClass != "power" {
		return nil, nil
	}

	if config.UniqueID == "" || config.StateTopic == "" {
		return nil, nil
	}

	vdevID := "esphome/" + strings.TrimSuffix(config.StateTopic, "/state")
	d := &VirtualDevice{
		ID:   vdevID,
		Type: VdevTypePowerUsage,
		MapperData: &ESPHomeMapperData{
			StateTopic: config.StateTopic,
			UniqueID:   config.UniqueID,
		},
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already known
	existing := m.devicesByStateTopic[config.StateTopic]
	found := false
	for _, e := range existing {
		if e.ID == d.ID {
			found = true
			break
		}
	}
	if !found {
		m.devicesByStateTopic[config.StateTopic] = append(m.devicesByStateTopic[config.StateTopic], d)
	}

	return []*VirtualDevice{d}, nil
}

// UpdateDevicesFromMessage parses state updates for discovered devices.
func (m *ESPHomeMapper) UpdateDevicesFromMessage(topic string, payload []byte) ([]*VirtualDeviceUpdate, error) {
	m.mu.RLock()
	devs, ok := m.devicesByStateTopic[topic]
	m.mu.RUnlock()
	if !ok {
		return nil, nil
	}

	valStr := string(payload)
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		log.Printf("[esphome] failed to parse state value %q as float: %v", valStr, err)
		return nil, nil
	}

	updates := make([]*VirtualDeviceUpdate, 0, len(devs))
	for _, d := range devs {
		updates = append(updates, &VirtualDeviceUpdate{
			Name:  d.ID,
			State: val,
		})
	}

	return updates, nil
}

// Control is a no-op for power sensors.
func (m *ESPHomeMapper) Control(vdev *VirtualDevice, state any, client mqtt.Client) error {
	return nil
}
