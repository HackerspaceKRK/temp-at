package main

import (
	"log"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ZamkonatorMapper struct {
	prefix string
}

// NewZamkonatorMapper creates a new Zamkonator mapper. prefix should include the trailing slash.
func NewZamkonatorMapper(prefix string) *ZamkonatorMapper {
	return &ZamkonatorMapper{prefix: prefix}
}

// SubscriptionTopics returns the door and lock state topics.
func (m *ZamkonatorMapper) SubscriptionTopics() []string {
	return []string{
		m.prefix + "+/door/state",
		m.prefix + "+/lock/state",
	}
}

// vdevID derives the vdev ID from a state topic by stripping the "/state" suffix.
func (m *ZamkonatorMapper) vdevID(topic string) (string, bool) {
	if !strings.HasPrefix(topic, m.prefix) || !strings.HasSuffix(topic, "/state") {
		return "", false
	}
	return strings.TrimSuffix(topic, "/state"), true
}

// DiscoverDevicesFromMessage creates a contact vdev for any state topic.
func (m *ZamkonatorMapper) DiscoverDevicesFromMessage(topic string, payload []byte) ([]*VirtualDevice, error) {
	id, ok := m.vdevID(topic)
	if !ok {
		return nil, nil
	}
	return []*VirtualDevice{{ID: id, Type: VdevTypeContact}}, nil
}

// UpdateDevicesFromMessage maps CLOSED/LOCKED to true and OPEN/UNLOCKED to false.
func (m *ZamkonatorMapper) UpdateDevicesFromMessage(topic string, payload []byte) ([]*VirtualDeviceUpdate, error) {
	id, ok := m.vdevID(topic)
	if !ok {
		return nil, nil
	}

	var closed bool
	switch val := string(payload); val {
	case "CLOSED", "LOCKED":
		closed = true
	case "OPEN", "UNLOCKED":
		closed = false
	default:
		log.Printf("[zamkonator] unknown state %q on topic %s", val, topic)
		return nil, nil
	}

	return []*VirtualDeviceUpdate{{Name: id, State: closed}}, nil
}

// Control is a no-op; Zamkonator devices are read-only sensors here.
func (m *ZamkonatorMapper) Control(vdev *VirtualDevice, state any, client mqtt.Client) error {
	return nil
}
