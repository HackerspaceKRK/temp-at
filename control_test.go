package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MockToken satisfies mqtt.Token
type MockToken struct {
	mqtt.Token
}

func (t *MockToken) Wait() bool {
	return true
}

func (t *MockToken) WaitTimeout(d time.Duration) bool {
	return true
}

func (t *MockToken) Error() error {
	return nil
}

// MockClient satisfies mqtt.Client
type MockClient struct {
	mqtt.Client
	PublishedTopic   string
	PublishedPayload []byte
}

func (m *MockClient) IsConnectionOpen() bool {
	return true
}

func (m *MockClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	m.PublishedTopic = topic
	switch v := payload.(type) {
	case []byte:
		m.PublishedPayload = v
	case string:
		m.PublishedPayload = []byte(v)
	}
	return &MockToken{}
}

func TestZigbee2MQTTMapper_Control(t *testing.T) {
	mapper := NewZigbee2MQTTMapper("zigbee2mqtt/")
	mockClient := &MockClient{}

	// Case 1: Standard relay
	vdev := &VirtualDevice{
		ID:   "relay/1",
		Type: VdevTypeRelay,
		MapperData: &Zigbee2MQTTMapperData{
			BaseTopic: "my-relay",
		},
	}

	err := mapper.Control(vdev, "ON", mockClient)
	if err != nil {
		t.Fatalf("Control failed: %v", err)
	}

	expectedTopic := "zigbee2mqtt/my-relay/set"
	if mockClient.PublishedTopic != expectedTopic {
		t.Errorf("Expected topic %s, got %s", expectedTopic, mockClient.PublishedTopic)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(mockClient.PublishedPayload, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if val, ok := payload["state"]; !ok || val != "ON" {
		t.Errorf("Expected payload {'state': 'ON'}, got %v", payload)
	}

	// Case 2: Relay with custom state key
	vdev2 := &VirtualDevice{
		ID:   "relay/2",
		Type: VdevTypeRelay,
		MapperData: &Zigbee2MQTTMapperData{
			BaseTopic: "my-relay-2",
			StateKey:  "state_left",
		},
	}

	err = mapper.Control(vdev2, "OFF", mockClient)
	if err != nil {
		t.Fatalf("Control failed: %v", err)
	}

	expectedTopic2 := "zigbee2mqtt/my-relay-2/set"
	if mockClient.PublishedTopic != expectedTopic2 {
		t.Errorf("Expected topic %s, got %s", expectedTopic2, mockClient.PublishedTopic)
	}

	if err := json.Unmarshal(mockClient.PublishedPayload, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if val, ok := payload["state_left"]; !ok || val != "OFF" {
		t.Errorf("Expected payload {'state_left': 'OFF'}, got %v", payload)
	}
}

func TestMQTTAdapter_ControlDevice_Validation(t *testing.T) {
	// Setup
	mgr := NewVdevManager()
	mockClient := &MockClient{}

	// Create adapter manually to avoid connection logic
	adapter := &MQTTAdapter{
		vdevMgr: mgr,
		client:  mockClient,
		mappers: []MQTTMapper{
			NewZigbee2MQTTMapper("zigbee2mqtt/"),
		},
	}

	// Add a relay device
	relayDev := &VirtualDevice{
		ID:   "relay/1",
		Type: VdevTypeRelay,
		MapperData: &Zigbee2MQTTMapperData{BaseTopic: "r1"},
	}
	mgr.AddDevices([]*VirtualDevice{relayDev})

	// Add a sensor device
	sensorDev := &VirtualDevice{
		ID:   "sensor/1",
		Type: VdevTypeTemperature,
		MapperData: &Zigbee2MQTTMapperData{BaseTopic: "s1"},
	}
	mgr.AddDevices([]*VirtualDevice{sensorDev})

	// Test 1: Valid Relay ON -> Success
	err := adapter.ControlDevice("relay/1", "ON")
	if err != nil {
		t.Errorf("Expected success for valid relay ON, got %v", err)
	}

	// Test 2: Valid Relay OFF -> Success
	err = adapter.ControlDevice("relay/1", "OFF")
	if err != nil {
		t.Errorf("Expected success for valid relay OFF, got %v", err)
	}

	// Test 3: Valid Relay lower case on -> Success (should notify user if strictness was required? Code converts to upper)
	err = adapter.ControlDevice("relay/1", "on")
	if err != nil {
		t.Errorf("Expected success for valid relay 'on', got %v", err)
	}

	// Test 4: Invalid State -> Error
	err = adapter.ControlDevice("relay/1", "INVALID")
	if err == nil {
		t.Errorf("Expected error for invalid state, got nil")
	} else if !strings.Contains(err.Error(), "must be ON or OFF") {
		t.Errorf("Expected 'must be ON or OFF' error, got: %v", err)
	}

	// Test 5: Not a Relay -> Error
	err = adapter.ControlDevice("sensor/1", "ON")
	if err == nil {
		t.Errorf("Expected error for non-relay device, got nil")
	} else if !strings.Contains(err.Error(), "not a relay") {
		t.Errorf("Expected 'not a relay' error, got: %v", err)
	}
}
