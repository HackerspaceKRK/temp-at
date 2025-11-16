package main

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
)

type Zigbee2MQTTMapperData struct {
	IEEEAddress string `json:"ieee_address"`
	StateKey    string `json:"state_key"`
	BaseTopic   string `json:"base_topic"`
	Endpoint    string `json:"endpoint"`
}

// Zigbee2MQTTMapper implements MQTTMapper for zigbee2mqtt messages.
type Zigbee2MQTTMapper struct {
	prefix string
	logger *log.Logger

	mu sync.RWMutex
	// devicesByBase stores discovered virtual devices keyed by their friendly base name.
	devicesByBase map[string][]*VirtualDevice
}

// NewZigbee2MQTTMapper creates a new mapper with the given topic prefix (e.g. "zigbee2mqtt/").
func NewZigbee2MQTTMapper(prefix string, logger *log.Logger) *Zigbee2MQTTMapper {
	if logger == nil {
		logger = log.Default()
	}
	if prefix == "" {
		prefix = "zigbee2mqtt/"
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	return &Zigbee2MQTTMapper{
		prefix:        prefix,
		logger:        logger,
		devicesByBase: make(map[string][]*VirtualDevice),
	}
}

// SubscriptionTopics returns topics needed for zigbee2mqtt discovery and updates.
func (m *Zigbee2MQTTMapper) SubscriptionTopics() []string {
	// We subscribe to bridge/devices for discovery and a wildcard for updates.
	return []string{
		m.prefix + "bridge/devices",
		m.prefix + "#",
	}
}

// DiscoverDevicesFromMessage parses the bridge/devices payload and builds virtual devices.
func (m *Zigbee2MQTTMapper) DiscoverDevicesFromMessage(topic string, payload []byte) ([]*VirtualDevice, error) {
	if topic != m.prefix+"bridge/devices" {
		return nil, nil
	}

	var rawDevices []json.RawMessage
	if err := json.Unmarshal(payload, &rawDevices); err != nil {
		return nil, err
	}

	discovered := []*VirtualDevice{}

	for _, raw := range rawDevices {
		var devMap map[string]any
		if err := json.Unmarshal(raw, &devMap); err != nil {
			// Skip individual device errors but continue processing.
			m.logger.Printf("[zigbee2mqtt] device entry unmarshal error: %v", err)
			continue
		}

		friendlyName, _ := devMap["friendly_name"].(string)
		if friendlyName == "" {
			continue
		}
		ieee, _ := devMap["ieee_address"].(string)

		defMap, _ := devMap["definition"].(map[string]any)
		exposes, _ := defMap["exposes"].([]any)
		if len(exposes) == 0 {
			continue
		}

		// Collect exposures of interest.
		var relayExposes []map[string]any
		var tempExposes []map[string]any
		var humidExposes []map[string]any

		for _, exp := range exposes {
			expMap, ok := exp.(map[string]any)
			if !ok {
				continue
			}
			if expMap["type"] == "switch" {
				relayExposes = append(relayExposes, expMap)
			}
			if expMap["type"] == "numeric" && expMap["property"] == "temperature" {
				tempExposes = append(tempExposes, expMap)
			}
			if expMap["type"] == "numeric" && expMap["property"] == "humidity" {
				humidExposes = append(humidExposes, expMap)
			}
		}

		// Build virtual relay devices.
		for _, ex := range relayExposes {
			endpoint := extractEndpointZigbee(ex)
			suffix := endpoint
			if suffix != "" {
				suffix = "/" + suffix
			}
			stateKey := ""
			if features, ok := ex["features"]; ok {
				if arr, ok := features.([]any); ok {
					for _, feature := range arr {
						fm, ok := feature.(map[string]any)
						if !ok {
							continue
						}
						if prop, ok := fm["property"]; ok {
							if s, ok := prop.(string); ok {
								stateKey = s
							}
						}
					}
				}
			}
			discovered = append(discovered, &VirtualDevice{
				ID:   friendlyName + suffix,
				Type: "relay",
				MapperData: &Zigbee2MQTTMapperData{
					BaseTopic:   friendlyName,
					Endpoint:    endpoint,
					IEEEAddress: ieee,
					StateKey:    stateKey,
				},
			})
		}

		// Build temperature virtual devices.
		for _, ex := range tempExposes {
			endpoint := extractEndpointZigbee(ex)
			discovered = append(discovered, &VirtualDevice{
				ID:   friendlyName + "/temperature",
				Type: "temperature",
				MapperData: &Zigbee2MQTTMapperData{
					BaseTopic:   friendlyName,
					Endpoint:    endpoint,
					IEEEAddress: ieee,
					StateKey:    "temperature",
				},
			})
		}

		// Build humidity virtual devices.
		for _, ex := range humidExposes {
			endpoint := extractEndpointZigbee(ex)
			discovered = append(discovered, &VirtualDevice{
				ID:   friendlyName + "/humidity",
				Type: "humidity",
				MapperData: &Zigbee2MQTTMapperData{
					BaseTopic:   friendlyName,
					Endpoint:    endpoint,
					IEEEAddress: ieee,
					StateKey:    "humidity",
				},
			})
		}
	}

	// Store discovered devices internally for update mapping.
	if len(discovered) > 0 {
		m.mu.Lock()
		for _, d := range discovered {
			base := d.MapperData.(*Zigbee2MQTTMapperData).BaseTopic
			// Ensure uniqueness by name.
			existingList := m.devicesByBase[base]
			exists := false
			for _, e := range existingList {
				if e.ID == d.ID {
					exists = true
					break
				}
			}
			if !exists {
				m.devicesByBase[base] = append(m.devicesByBase[base], d)
			}
		}
		m.mu.Unlock()
	}

	return discovered, nil
}

// UpdateDevicesFromMessage parses state update payloads for existing virtual devices.
func (m *Zigbee2MQTTMapper) UpdateDevicesFromMessage(topic string, payload []byte) ([]*VirtualDeviceUpdate, error) {
	if !strings.HasPrefix(topic, m.prefix) {
		return nil, nil
	}
	// Ignore bridge messages.
	if strings.HasPrefix(topic, m.prefix+"bridge/") {
		return nil, nil
	}

	baseName := strings.TrimPrefix(topic, m.prefix)
	m.mu.RLock()
	devs := m.devicesByBase[baseName]
	m.mu.RUnlock()
	if len(devs) == 0 {
		return nil, nil
	}

	// Parse JSON payload.
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}

	updates := []*VirtualDeviceUpdate{}
	for _, d := range devs {
		if d.MapperData == nil {
			continue
		}
		if val, ok := d.MapperData.(*Zigbee2MQTTMapperData); ok && val.StateKey != "" {
			updates = append(updates, &VirtualDeviceUpdate{
				Name:  d.ID,
				State: parsed[val.StateKey],
			})
		} else {
			// Some devices (e.g. relay) might represent state as uppercase "state"
			// If StateKey was not set we could attempt fallback keys here if needed in future.
		}
	}

	return updates, nil
}

// extractEndpointZigbee attempts to read the endpoint field from an exposure map.
func extractEndpointZigbee(ex map[string]any) string {
	if ep, ok := ex["endpoint"].(string); ok {
		return ep
	}
	return ""
}
