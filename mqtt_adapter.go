package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// device base topics (without virtualization).
func deviceStateSetTopic(base string) string { return "zigbee2mqtt/" + base + "/set" }
func deviceStateTopic(base string) string    { return "zigbee2mqtt/" + base }

// VirtualDevice represents a single controllable/readable capability broken out
// from a physical Zigbee device (e.g. multi-relay or multi-sensor).
type VirtualDevice struct {
	// Name is the unique virtual name (base_name plus suffix).
	Name string `json:"name"`
	// BaseName is the original physical device friendly_name from Zigbee2MQTT.
	BaseName string `json:"base_name"`
	// Type: "relay", "temperature", "humidity", "person"
	Type string `json:"type"`
	// Endpoint identifier if applicable (e.g. "1", "2" for multi-channel relays).
	Endpoint string `json:"endpoint,omitempty"`
	// IEEE address of the underlying device (for reference).
	IEEEAddress string `json:"ieee_address,omitempty"`

	// StateKey is the JSON key used to extract the state for this virtual device from the message payload.
	StateKey string `json:"state_key,omitempty"`

	// Current state of the given device
	// true/false for relays, float for temperature, humidity, int for person (count)
	State any `json:"state,omitempty"`
}

// MQTTAdapter manages connection and maintains a list of virtual devices.
type MQTTAdapter struct {
	client mqtt.Client
	logger *log.Logger
	config *Config

	started atomicBool

	virtualMu      sync.RWMutex
	virtualDevices []*VirtualDevice

	OnVirtualDeviceUpdated func(name string)

	deviceSubscriptions map[string]bool

	zigbee2MqttPrefix string
	frigatePrefix     string
}

// atomicBool (simple mutex-backed boolean) avoids importing sync/atomic for minimal usage.
type atomicBool struct {
	mu  sync.Mutex
	val bool
}

func (b *atomicBool) Set(v bool) {
	b.mu.Lock()
	b.val = v
	b.mu.Unlock()
}
func (b *atomicBool) Get() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.val
}

// NewMQTTAdapter creates and connects the MQTT client; subscribes to the devices listing.
func NewMQTTAdapter(cfg *Config, logger *log.Logger) (*MQTTAdapter, error) {
	if logger == nil {
		logger = log.Default()
	}
	a := &MQTTAdapter{
		logger:              logger,
		config:              cfg,
		deviceSubscriptions: make(map[string]bool),
		zigbee2MqttPrefix:   "zigbee2mqtt/",
		frigatePrefix:       "frigate/",
	}

	opts, err := a.buildClientOptions(cfg)
	if err != nil {
		return nil, err
	}

	opts.OnConnect = func(c mqtt.Client) {
		a.logger.Printf("[mqtt] connected to %s", cfg.MQTT.Broker)
		if err := a.subscribeDevicesTopic(); err != nil {
			a.logger.Printf("[mqtt] failed to subscribe devices topic: %v", err)
		}

		frigateEnabledTopic := a.frigatePrefix + "+/enabled/state"
		a.logger.Printf("[mqtt] subscribing to frigate camera enabled topics: %s", frigateEnabledTopic)
		token := a.client.Subscribe(frigateEnabledTopic, 0, a.handleFrigateEnabledMessage)
		if !token.WaitTimeout(5 * time.Second) {
			a.logger.Printf("[mqtt] subscription timeout for %s", frigateEnabledTopic)
		} else if err := token.Error(); err != nil {
			a.logger.Printf("[mqtt] failed to subscribe to %s: %v", frigateEnabledTopic, err)
		}

		frigatePersonTopic := a.frigatePrefix + "+/person/active"
		a.logger.Printf("[mqtt] subscribing to frigate camera person active topics: %s", frigatePersonTopic)
		token = a.client.Subscribe(frigatePersonTopic, 0, a.handleFrigatePersonMessage)
		if !token.WaitTimeout(5 * time.Second) {
			a.logger.Printf("[mqtt] subscription timeout for %s", frigatePersonTopic)
		} else if err := token.Error(); err != nil {
			a.logger.Printf("[mqtt] failed to subscribe to %s: %v", frigatePersonTopic, err)
		}
	}

	a.client = mqtt.NewClient(opts)
	token := a.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return nil, errors.New("mqtt connect timeout after 10s")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect failed: %w", err)
	}
	a.started.Set(true)
	return a, nil
}

func (a *MQTTAdapter) buildClientOptions(cfg *Config) (*mqtt.ClientOptions, error) {
	broker := strings.TrimSpace(cfg.MQTT.Broker)
	if broker == "" {
		return nil, errors.New("empty mqtt broker in config")
	}
	if !strings.Contains(broker, "://") {
		broker = "tcp://" + broker
	}

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(fmt.Sprintf("temp-at-%d", time.Now().UnixNano())).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetKeepAlive(30 * time.Second).
		SetConnectTimeout(8 * time.Second).
		SetOrderMatters(false)

	if cfg.MQTT.Username != "" {
		opts.SetUsername(cfg.MQTT.Username)
	}
	if cfg.MQTT.Password != "" {
		opts.SetPassword(cfg.MQTT.Password)
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		if err != nil {
			a.logger.Printf("[mqtt] connection lost: %v", err)
		} else {
			a.logger.Printf("[mqtt] connection lost")
		}
	}
	return opts, nil
}

func (a *MQTTAdapter) subscribeDevicesTopic() error {
	if a.client == nil {
		return errors.New("mqtt client not initialized")
	}
	token := a.client.Subscribe(a.zigbee2MqttPrefix+"bridge/devices", 0, a.handleDevicesMessage)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscription timeout for %s", a.zigbee2MqttPrefix+"bridge/devices")
	}
	return token.Error()
}

// handleDevicesMessage parses the devices payload and builds virtual devices.
func (a *MQTTAdapter) handleDevicesMessage(_ mqtt.Client, msg mqtt.Message) {
	var rawDevices []json.RawMessage
	if err := json.Unmarshal(msg.Payload(), &rawDevices); err != nil {
		a.logger.Printf("[mqtt] devices payload unmarshal error: %v", err)
		return
	}

	a.virtualMu.Lock()
	defer a.virtualMu.Unlock()

	for i, raw := range rawDevices {
		var devMap map[string]any
		if err := json.Unmarshal(raw, &devMap); err != nil {
			a.logger.Printf("[mqtt] device[%d] map unmarshal error: %v", i, err)
			continue
		}

		friendlyName, _ := devMap["friendly_name"].(string)
		if friendlyName == "" {
			continue
		}
		ieee, _ := devMap["ieee_address"].(string)

		defMap, _ := devMap["definition"].(map[string]any)
		exposes, _ := defMap["exposes"].([]any)

		// Collect exposures of interest.
		var relayExposes []map[string]any
		var tempExposes []map[string]any
		var humidExposes []map[string]any

		for _, exp := range exposes {
			exp := exp.(map[string]any)
			if exp["type"] == "switch" {
				relayExposes = append(relayExposes, exp)
			}

			if exp["type"] == "numeric" && exp["property"] == "temperature" {
				tempExposes = append(tempExposes, exp)
			}

			if exp["type"] == "numeric" && exp["property"] == "humidity" {
				humidExposes = append(humidExposes, exp)
			}
		}

		// Build virtual relay devices.
		for _, ex := range relayExposes {
			endpoint := extractEndpoint(ex)
			suffix := endpoint
			if suffix != "" {
				suffix = "/" + suffix
			}
			stateKey := ""
			if features, ok := ex["features"]; ok {
				for _, feature := range features.([]any) {
					feature := feature.(map[string]any)
					if prop, ok := feature["property"]; ok {
						stateKey = prop.(string)
					}
				}
			}
			a.virtualDevices = append(a.virtualDevices, &VirtualDevice{
				Name:        friendlyName + suffix,
				BaseName:    friendlyName,
				Type:        "relay",
				Endpoint:    endpoint,
				IEEEAddress: ieee,
				StateKey:    stateKey,
			})
		}

		// Build temperature virtual devices.
		for _, ex := range tempExposes {
			endpoint := extractEndpoint(ex)
			nameSuffix := "/temperature"

			a.virtualDevices = append(a.virtualDevices, &VirtualDevice{
				Name:        friendlyName + nameSuffix,
				BaseName:    friendlyName,
				Type:        "temperature",
				Endpoint:    endpoint,
				IEEEAddress: ieee,
				StateKey:    "temperature",
			})
		}

		// Build humidity virtual devices.
		for _, ex := range humidExposes {
			endpoint := extractEndpoint(ex)
			nameSuffix := "/humidity"

			a.virtualDevices = append(a.virtualDevices, &VirtualDevice{
				Name:        friendlyName + nameSuffix,
				BaseName:    friendlyName,
				Type:        "humidity",
				Endpoint:    endpoint,
				IEEEAddress: ieee,
				StateKey:    "humidity",
			})
		}
	}

	for _, device := range a.virtualDevices {
		if _, ok := a.deviceSubscriptions[device.BaseName]; !ok {
			a.deviceSubscriptions[device.BaseName] = true
			topic := a.zigbee2MqttPrefix + device.BaseName
			token := a.client.Subscribe(topic, 0, a.handleDeviceMessage)
			if !token.WaitTimeout(5 * time.Second) {
				a.logger.Printf("[mqtt] failed to subscribe to %s", topic)
			}
			if err := token.Error(); err != nil {
				a.logger.Printf("[mqtt] failed to subscribe to %s: %v", topic, err)
			}
		}
	}

}

func (a *MQTTAdapter) handleDeviceMessage(_ mqtt.Client, msg mqtt.Message) {

	message := string(msg.Payload())
	var parsed map[string]any
	if err := json.Unmarshal([]byte(message), &parsed); err != nil {
		a.logger.Printf("[mqtt] failed to parse message from %s: %v", msg.Topic(), err)
		return
	}

	updatedDeviceNames := []string{}
	func() {

		a.virtualMu.Lock()
		defer a.virtualMu.Unlock()
		for _, device := range a.virtualDevices {
			if a.zigbee2MqttPrefix+device.BaseName == msg.Topic() {
				device.State = parsed[device.StateKey]
				updatedDeviceNames = append(updatedDeviceNames, device.Name)

			}
		}
	}()
	for _, name := range updatedDeviceNames {
		if a.OnVirtualDeviceUpdated != nil {
			a.OnVirtualDeviceUpdated(name)
		}
	}
}

// handleFrigateEnabledMessage handles messages on frigate/<camera>/enabled.
// For now, just extracts camera name and logs receipt; no further action.
func (a *MQTTAdapter) handleFrigateEnabledMessage(_ mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	if !strings.HasPrefix(topic, a.frigatePrefix) {
		return
	}
	a.virtualMu.Lock()
	defer a.virtualMu.Unlock()
	rest := strings.TrimPrefix(topic, a.frigatePrefix) // e.g. "front_door/enabled"
	parts := strings.Split(rest, "/")
	if len(parts) >= 2 && parts[1] == "enabled" {
		cameraName := parts[0]
		virtName := fmt.Sprintf("person/%s", cameraName)

		for _, dev := range a.virtualDevices {
			if dev.BaseName == virtName {
				return // Device already exists
			}
		}
		// Create a virtual device called person/<cameraName>
		personDevice := &VirtualDevice{
			Name:     virtName,
			BaseName: fmt.Sprintf("%s/person/active", cameraName),
			Type:     "person",
		}

		a.virtualDevices = append(a.virtualDevices, personDevice)

	}
}

// handleFrigatePersonMessage handles frigate person messages.
func (a *MQTTAdapter) handleFrigatePersonMessage(_ mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	if !strings.HasPrefix(topic, a.frigatePrefix) {
		return
	}
	updatedDeviceNames := []string{}
	func() {
		a.virtualMu.Lock()
		defer a.virtualMu.Unlock()
		rest := strings.TrimPrefix(topic, a.frigatePrefix)
		for _, dev := range a.virtualDevices {
			if dev.BaseName == rest {
				intVal, _ := strconv.Atoi(string(msg.Payload()))
				dev.State = intVal
				updatedDeviceNames = append(updatedDeviceNames, dev.Name)

				return
			}
		}
	}()
	if a.OnVirtualDeviceUpdated != nil {
		for _, name := range updatedDeviceNames {
			a.OnVirtualDeviceUpdated(name)
		}
	}
}

// extractEndpoint attempts to read the endpoint field from an exposure map.
func extractEndpoint(ex map[string]any) string {
	if ep, ok := ex["endpoint"].(string); ok {
		return ep
	}
	return ""
}

// VirtualDevices returns a snapshot list of current virtual devices.
func (a *MQTTAdapter) VirtualDevices() []*VirtualDevice {
	a.virtualMu.RLock()
	defer a.virtualMu.RUnlock()
	cp := make([]*VirtualDevice, len(a.virtualDevices))
	for i, dev := range a.virtualDevices {
		// deep copy
		var newDev = *dev
		cp[i] = &newDev
	}
	return cp
}

// Close disconnects MQTT client.
func (a *MQTTAdapter) Close() {
	if a.client != nil && a.client.IsConnectionOpen() {
		a.client.Disconnect(250)
		a.logger.Printf("[mqtt] disconnected")
	}
}

// IsConnected returns true if underlying MQTT client is connected.
func (a *MQTTAdapter) IsConnected() bool {
	return a.client != nil && a.client.IsConnectionOpen()
}
