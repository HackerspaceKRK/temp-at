package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// DevicesTopic is the Zigbee2MQTT devices list topic that returns a JSON array of devices.
const DevicesTopic = "zigbee2mqtt/bridge/devices"

// device base topics (without virtualization).
func deviceStateSetTopic(base string) string { return "zigbee2mqtt/" + base + "/set" }
func deviceStateTopic(base string) string    { return "zigbee2mqtt/" + base }

// VirtualDevice represents a single controllable/readable capability broken out
// from a physical Zigbee device (e.g. multi-relay or multi-sensor).
//
// Examples of Name generation:
//
//	Physical device friendly_name: lights/elelab
//	Two relay endpoints -> lights/elelab/l1 and lights/elelab/l2
//	Temperature sensor   -> sensor/desk/temperature
//	Humidity sensor      -> sensor/desk/humidity
type VirtualDevice struct {
	// Name is the unique virtual name (base_name plus suffix).
	Name string `json:"name"`
	// BaseName is the original physical device friendly_name from Zigbee2MQTT.
	BaseName string `json:"base_name"`
	// Type: "relay", "temperature", "humidity".
	Type string `json:"capability"`
	// Endpoint identifier if applicable (e.g. "1", "2" for multi-channel relays).
	Endpoint string `json:"endpoint,omitempty"`
	// IEEE address of the underlying device (for reference).
	IEEEAddress string `json:"ieee_address,omitempty"`
}

// MQTTAdapter manages connection and maintains a list of virtual devices.
// It does NOT keep telemetry state caches; it only discovers structure/capabilities.
type MQTTAdapter struct {
	client mqtt.Client
	logger *log.Logger
	config *Config

	started atomicBool

	virtualMu      sync.RWMutex
	virtualDevices []VirtualDevice

	onVirtualDevicesUpdated func([]VirtualDevice)
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
		logger: logger,
		config: cfg,
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
	token := a.client.Subscribe(DevicesTopic, 0, a.handleDevicesMessage)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscription timeout for %s", DevicesTopic)
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

	var virtual []VirtualDevice

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
			// log.Printf("%#v", exp["type"])
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
			virtual = append(virtual, VirtualDevice{
				Name:        friendlyName + suffix,
				BaseName:    friendlyName,
				Type:        "relay",
				Endpoint:    endpoint,
				IEEEAddress: ieee,
			})
		}

		// Build temperature virtual devices.
		for _, ex := range tempExposes {
			endpoint := extractEndpoint(ex)
			nameSuffix := "/temperature"

			virtual = append(virtual, VirtualDevice{
				Name:        friendlyName + nameSuffix,
				BaseName:    friendlyName,
				Type:        "temperature",
				Endpoint:    endpoint,
				IEEEAddress: ieee,
			})
		}

		// Build humidity virtual devices.
		for _, ex := range humidExposes {
			endpoint := extractEndpoint(ex)
			nameSuffix := "/humidity"

			virtual = append(virtual, VirtualDevice{
				Name:        friendlyName + nameSuffix,
				BaseName:    friendlyName,
				Type:        "humidity",
				Endpoint:    endpoint,
				IEEEAddress: ieee,
			})
		}
	}

	a.virtualMu.Lock()
	a.virtualDevices = virtual
	a.virtualMu.Unlock()

	if a.onVirtualDevicesUpdated != nil {
		// Provide a copy.
		cp := make([]VirtualDevice, len(virtual))
		copy(cp, virtual)
		a.onVirtualDevicesUpdated(cp)
	}

	a.logger.Printf("[mqtt] virtual devices updated (%d)", len(virtual))
}

// extractEndpoint attempts to read the endpoint field from an exposure map.
func extractEndpoint(ex map[string]any) string {
	if ep, ok := ex["endpoint"].(string); ok {
		return ep
	}
	return ""
}

// VirtualDevices returns a snapshot list of current virtual devices.
func (a *MQTTAdapter) VirtualDevices() []VirtualDevice {
	a.virtualMu.RLock()
	defer a.virtualMu.RUnlock()
	cp := make([]VirtualDevice, len(a.virtualDevices))
	copy(cp, a.virtualDevices)
	return cp
}

// OnVirtualDevicesUpdated registers a callback invoked whenever virtual devices list changes.
func (a *MQTTAdapter) OnVirtualDevicesUpdated(cb func([]VirtualDevice)) {
	a.onVirtualDevicesUpdated = cb
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
