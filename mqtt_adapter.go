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
	// Control attempts to control a specific virtual device.
	Control(vdev *VirtualDevice, state any, client mqtt.Client) error
} // MQTTMapper

// MQTTAdapter adapts mqtt messages coming from multiple sources (e.g. Zigbee2MQTT, Frigate)
// into a unified list of VirtualDevice objects managed by VdevManager.
type MQTTAdapter struct {
	client mqtt.Client
	config *Config

	started atomicBool
	// Virtual device manager extracted from previous in-struct logic.
	vdevMgr *VdevManager
	mappers []MQTTMapper

	// deviceSettings maps device ID to its configuration (for prohibited control etc)
	deviceSettings map[string]EntityConfig
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
func NewMQTTAdapter(cfg *Config, vdevMgr *VdevManager) (*MQTTAdapter, error) {

	a := &MQTTAdapter{

		config:         cfg,
		vdevMgr:        vdevMgr,
		deviceSettings: make(map[string]EntityConfig),
	}

	// Index device configurations for fast lookup
	for _, room := range cfg.Rooms {
		for _, entity := range room.Entities {
			a.deviceSettings[entity.ID] = entity
		}
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
		NewZigbee2MQTTMapper("zigbee2mqtt/"),
		NewFrigateMapper("frigate/"),
		NewESPHomeMapper(a.deviceSettings),
	}

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("[mqtt] connected to %s", cfg.MQTT.Broker)
		a.subscribeAllMapperTopics()
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		if err != nil {
			log.Printf("[mqtt] connection lost: %v", err)
		} else {
			log.Printf("[mqtt] connection lost")
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
		log.Printf("[mqtt] client is nil, cannot subscribe")
		return
	}

	for _, mapper := range a.mappers {
		for _, topic := range mapper.SubscriptionTopics() {
			topic := topic // capture loop variable
			log.Printf("[mqtt] subscribing to %s", topic)
			token := a.client.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
				a.handleMapperMessage(mapper, msg.Topic(), msg.Payload())
			})
			if !token.WaitTimeout(5 * time.Second) {
				log.Printf("[mqtt] subscription timeout for %s", topic)
			} else if err := token.Error(); err != nil {
				log.Printf("[mqtt] failed to subscribe to %s: %v", topic, err)
			}
		}
	}
}

// handleMapperMessage invokes discovery and update logic on a mapper and mutates virtual devices accordingly.
func (a *MQTTAdapter) handleMapperMessage(mapper MQTTMapper, topic string, payload []byte) {
	// Discovery
	discovered, derr := mapper.DiscoverDevicesFromMessage(topic, payload)
	if derr != nil {
		log.Printf("[mqtt] discovery error on topic %s: %v", topic, derr)
	}
	if len(discovered) > 0 {
		// Enrich discovered devices with config data
		for _, d := range discovered {
			if cfg, ok := a.deviceSettings[d.ID]; ok {
				d.ProhibitControl = cfg.ProhibitControl
			}
		}
		a.vdevMgr.AddDevices(discovered)
	}

	// Updates
	updates, uerr := mapper.UpdateDevicesFromMessage(topic, payload)
	if uerr != nil {
		log.Printf("[mqtt] update error on topic %s: %v", topic, uerr)
	}
	if len(updates) > 0 {
		// VdevManager handles callback invocation.
		a.vdevMgr.ApplyUpdates(updates)
	}
}

// Close disconnects MQTT client.
func (a *MQTTAdapter) Close() {
	if a.client != nil && a.client.IsConnectionOpen() {
		a.client.Disconnect(250)
		log.Printf("[mqtt] disconnected")
	}
}

// IsConnected returns true if underlying MQTT client is connected.
func (a *MQTTAdapter) IsConnected() bool {
	return a.client != nil && a.client.IsConnectionOpen()
}

// ControlDevice attempts to find the device and the responsible mapper to send a control command.
func (a *MQTTAdapter) ControlDevice(deviceID string, state any) error {
	if !a.IsConnected() {
		return errors.New("MQTT client not connected")
	}

	// 1. Retrieve device to check type and mapper data.
	a.vdevMgr.mu.RLock()
	var targetDev *VirtualDevice
	for _, dev := range a.vdevMgr.devices {
		if dev.ID == deviceID {
			targetDev = dev
			break
		}
	}
	a.vdevMgr.mu.RUnlock()

	if targetDev == nil {
		return fmt.Errorf("device %s not found", deviceID)
	}

	// 2. Validation: Relay only
	if targetDev.Type != VdevTypeRelay {
		return fmt.Errorf("device %s is not a relay (type: %s)", deviceID, targetDev.Type)
	}

	// 2.5 Validation: ProhibitControl
	if targetDev.ProhibitControl {
		return fmt.Errorf("control is prohibited for device %s", deviceID)
	}

	// 3. Validation: State must be ON or OFF
	stateStr, ok := state.(string)
	if !ok {
		return errors.New("state must be a string")
	}
	upperState := strings.ToUpper(stateStr)
	if upperState != "ON" && upperState != "OFF" {
		return fmt.Errorf("invalid state %q; must be ON or OFF", stateStr)
	}

	// 4. Iterate mappers to find who owns this device (or just try all, since they check internally).
	// Ideally, we'd know which mapper owns it, but the current architecture lazily checks payload/topic.
	// Since we have the device struct, we can pass it to mappers and see if they recognize their own metadata.
	// However, `Control` assumes the mapper knows how to handle it.
	// The `Zigbee2MQTTMapper` checks `vdev.MapperData.(*Zigbee2MQTTMapperData)`.
	// So safe to iterate all.

	for _, mapper := range a.mappers {
		// We pass strict "ON" or "OFF" to ensure consistency.
		if err := mapper.Control(targetDev, upperState, a.client); err != nil {
			return err
		}
	}

	return nil
}
