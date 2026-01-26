package main

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
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

	mu sync.RWMutex
	// devicesByBase stores discovered virtual devices keyed by their friendly base name.
	devicesByBase map[string][]*VirtualDevice
}

// NewZigbee2MQTTMapper creates a new mapper with the given topic prefix (e.g. "zigbee2mqtt/").
func NewZigbee2MQTTMapper(prefix string) *Zigbee2MQTTMapper {

	return &Zigbee2MQTTMapper{
		prefix:        prefix,
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

type z2mExposureConfig struct {
	VdevType  VdevType
	IDsomefix string
	StateKey  string
}

var z2mSensorConfigs = map[string]z2mExposureConfig{
	"numeric:temperature": {VdevTypeTemperature, "/temperature", "temperature"},
	"numeric:humidity":    {VdevTypeHumidity, "/humidity", "humidity"},
	"numeric:co":          {VdevTypeCo, "/co", "co"},
	"numeric:gas_value":   {VdevTypeGas, "/gas", "gas_value"},
	"binary:contact":      {VdevTypeContact, "/contact", "contact"},
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
			log.Printf("[zigbee2mqtt] device entry unmarshal error: %v", err)
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

		for _, exp := range exposes {
			expMap, ok := exp.(map[string]any)
			if !ok {
				continue
			}

			expType, _ := expMap["type"].(string)

			// Handle switches (special case for relays with endpoints)
			if expType == "switch" {
				endpoint := extractEndpointZigbee(expMap)
				suffix := endpoint
				if suffix != "" {
					suffix = "/" + suffix
				}
				stateKey := ""
				if features, ok := expMap["features"]; ok {
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
					Type: VdevTypeRelay,
					MapperData: &Zigbee2MQTTMapperData{
						BaseTopic:   friendlyName,
						Endpoint:    endpoint,
						IEEEAddress: ieee,
						StateKey:    stateKey,
					},
				})
				continue
			}

			// Handle general sensors from the map
			property, _ := expMap["property"].(string)
			mapKey := expType + ":" + property

			if config, ok := z2mSensorConfigs[mapKey]; ok {
				endpoint := extractEndpointZigbee(expMap)
				discovered = append(discovered, &VirtualDevice{
					ID:   friendlyName + config.IDsomefix,
					Type: config.VdevType,
					MapperData: &Zigbee2MQTTMapperData{
						BaseTopic:   friendlyName,
						Endpoint:    endpoint,
						IEEEAddress: ieee,
						StateKey:    config.StateKey,
					},
				})
			}
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

// Control publishes a set message to the zigbee2mqtt device.
func (m *Zigbee2MQTTMapper) Control(vdev *VirtualDevice, state any, client mqtt.Client) error {
	mapperData, ok := vdev.MapperData.(*Zigbee2MQTTMapperData)
	if !ok {
		return nil // Not managed by this mapper
	}

	// Construct topic: prefix + friendlyName + /set
	// e.g. zigbee2mqtt/my-relay/set
	topic := m.prefix + mapperData.BaseTopic + "/set"

	// Construct payload.
	// If StateKey is present, use it: e.g. {"state_l2": "ON"}
	// If Endpoint is present, some z2m setups might need it, but usually sending to friendly_name/set with {"state_bottom": "ON"} etc is enough.
	payloadMap := map[string]any{}

	key := mapperData.StateKey
	if key == "" {
		key = "state"
	}
	payloadMap[key] = state

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return err
	}

	log.Printf("[zigbee2mqtt] controlling %s via topic %s: %s", vdev.ID, topic, string(payloadBytes))
	token := client.Publish(topic, 0, false, payloadBytes)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}
