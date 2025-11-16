package main

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// VirtualDevice represents a single controllable/readable capability broken out
// from a physical Zigbee device (e.g. multi-relay or multi-sensor).
type VirtualDevice struct {
	// Name is the unique virtual name (base_name plus suffix).
	Name string `json:"name"`
	// BaseName is the original physical device friendly_name from Zigbee2MQTT or logical base for other services.
	BaseName string `json:"base_name"`
	// Type: "relay", "temperature", "humidity", "person", etc.
	Type string `json:"type"`
	// Endpoint identifier if applicable (e.g. "1", "2" for multi-channel relays).
	Endpoint string `json:"endpoint,omitempty"`
	// IEEE address of the underlying device (for reference) if available.
	IEEEAddress string `json:"ieee_address,omitempty"`
	// StateKey is the JSON key used to extract the state for this virtual device from the message payload.
	StateKey string `json:"state_key,omitempty"`
	// Current state of the given device (bool, float64, int, etc).
	State any `json:"state,omitempty"`
}

type VirtualDeviceUpdate struct {
	Name  string `json:"name"`
	State any    `json:"state,omitempty"`
}

// MQTTMapper defines the contract for mapping MQTT messages into virtual devices.
//
// Implementations should:
// - Return the list of topics they need to subscribe to.
// - Parse discovery style messages into VirtualDevice objects.
// - Parse update messages into VirtualDeviceUpdate objects.
//
// A single incoming message may produce both newly discovered devices and updates.
// If a mapper does not discover or update anything for a given message it should return nil slices.
type MQTTMapper interface {
	// SubscriptionTopics returns MQTT topics (wildcards allowed) to subscribe for this mapper.
	SubscriptionTopics() []string
	// DiscoverDevicesFromMessage attempts to discover new devices from an incoming message.
	DiscoverDevicesFromMessage(topic string, payload []byte) ([]*VirtualDevice, error)
	// UpdateDevicesFromMessage attempts to extract state updates from an incoming message.
	UpdateDevicesFromMessage(topic string, payload []byte) ([]*VirtualDeviceUpdate, error)
}

// MQTTAdapter adapts mqtt messages coming from multiple sources (e.g. Zigbee2MQTT, Frigate)
// into a unified list of VirtualDevice objects.
type MQTTAdapter struct {
	client mqtt.Client
	logger *log.Logger
	config *Config

	started atomicBool

	virtualMu      sync.RWMutex
	virtualDevices []*VirtualDevice

	OnVirtualDeviceUpdated func(name string)

	zigbee2MqttPrefix string
	frigatePrefix     string

	mappers []MQTTMapper
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

// NewMQTTAdapter creates and connects the MQTT client; registers mapper subscriptions.
func NewMQTTAdapter(cfg *Config, logger *log.Logger) (*MQTTAdapter, error) {
	if logger == nil {
		logger = log.Default()
	}
	a := &MQTTAdapter{
		logger:            logger,
		config:            cfg,
		zigbee2MqttPrefix: "zigbee2mqtt/",
		frigatePrefix:     "frigate/",
	}

	// Build client options first.
	opts, err := a.buildClientOptions(cfg)
	if err != nil {
		return nil, err
	}

	// Instantiate mapper implementations.
	// These constructors must be provided by:
	// - mqtt_mapper_zigbee2mqtt.go
	// - mqtt_mapper_frigate.go
	a.mappers = []MQTTMapper{
		NewZigbee2MQTTMapper(a.zigbee2MqttPrefix, a.logger),
		NewFrigateMapper(a.frigatePrefix, a.logger),
	}

	opts.OnConnect = func(c mqtt.Client) {
		a.logger.Printf("[mqtt] connected to %s", cfg.MQTT.Broker)
		a.subscribeAllMapperTopics()
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		if err != nil {
			a.logger.Printf("[mqtt] connection lost: %v", err)
		} else {
			a.logger.Printf("[mqtt] connection lost")
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
	return opts, nil
}

// subscribeAllMapperTopics subscribes to all topics declared by each mapper implementation.
func (a *MQTTAdapter) subscribeAllMapperTopics() {
	if a.client == nil {
		a.logger.Printf("[mqtt] client is nil, cannot subscribe")
		return
	}

	for _, mapper := range a.mappers {
		for _, topic := range mapper.SubscriptionTopics() {
			topic := topic // capture loop variable
			a.logger.Printf("[mqtt] subscribing to %s", topic)
			token := a.client.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
				a.handleMapperMessage(mapper, msg.Topic(), msg.Payload())
			})
			if !token.WaitTimeout(5 * time.Second) {
				a.logger.Printf("[mqtt] subscription timeout for %s", topic)
			} else if err := token.Error(); err != nil {
				a.logger.Printf("[mqtt] failed to subscribe to %s: %v", topic, err)
			}
		}
	}
}

// handleMapperMessage invokes discovery and update logic on a mapper and mutates virtual devices accordingly.
func (a *MQTTAdapter) handleMapperMessage(mapper MQTTMapper, topic string, payload []byte) {
	// Discovery
	discovered, derr := mapper.DiscoverDevicesFromMessage(topic, payload)
	if derr != nil {
		a.logger.Printf("[mqtt] discovery error on topic %s: %v", topic, derr)
	}
	if len(discovered) > 0 {
		a.addVirtualDevices(discovered)
	}

	// Updates
	updates, uerr := mapper.UpdateDevicesFromMessage(topic, payload)
	if uerr != nil {
		a.logger.Printf("[mqtt] update error on topic %s: %v", topic, uerr)
	}
	if len(updates) > 0 {
		updatedNames := a.applyUpdates(updates)
		if a.OnVirtualDeviceUpdated != nil {
			for _, name := range updatedNames {
				a.OnVirtualDeviceUpdated(name)
			}
		}
	}
}

// addVirtualDevices adds new devices if their Name is not already present.
func (a *MQTTAdapter) addVirtualDevices(devs []*VirtualDevice) {
	a.virtualMu.Lock()
	defer a.virtualMu.Unlock()

	existing := make(map[string]struct{}, len(a.virtualDevices))
	for _, d := range a.virtualDevices {
		existing[d.Name] = struct{}{}
	}
	for _, d := range devs {
		if d == nil || d.Name == "" {
			continue
		}
		if _, found := existing[d.Name]; found {
			continue
		}
		a.virtualDevices = append(a.virtualDevices, d)
	}
}

// applyUpdates applies state updates and returns the list of device names that changed.
func (a *MQTTAdapter) applyUpdates(updates []*VirtualDeviceUpdate) []string {
	updatedNames := []string{}
	a.virtualMu.Lock()
	defer a.virtualMu.Unlock()

	// Build index by name for O(1) lookups.
	index := make(map[string]*VirtualDevice, len(a.virtualDevices))
	for _, d := range a.virtualDevices {
		index[d.Name] = d
	}

	for _, upd := range updates {
		if upd == nil || upd.Name == "" {
			continue
		}
		if dev, ok := index[upd.Name]; ok {
			dev.State = upd.State
			updatedNames = append(updatedNames, dev.Name)
		}
	}
	return updatedNames
}

// VirtualDevices returns a snapshot list of current virtual devices.
func (a *MQTTAdapter) VirtualDevices() []*VirtualDevice {
	a.virtualMu.RLock()
	defer a.virtualMu.RUnlock()
	cp := make([]*VirtualDevice, len(a.virtualDevices))
	for i, dev := range a.virtualDevices {
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
