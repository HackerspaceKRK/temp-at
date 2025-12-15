package main

import (
	"log"
	"strconv"
	"strings"
)

// FrigateMapperData is stored in the MapperData field of VirtualDevice for FrigateMapper devices.
type FrigateMapperData struct {
	CameraName string `json:"camera_name"`
}

// FrigateMapper implements MQTTMapper for Frigate camera person detection.
//
// Discovery:
//
//	Subscribes to:   frigate/<camera>/enabled/state
//	When received, creates a VirtualDevice:
//	    Name:      person/<camera>
//	    BaseName:  <camera>/person/active   (used to match update topic suffix)
//	    Type:      person
//
// Updates:
//
//	Subscribes to:   frigate/<camera>/person/active
//	Payload is expected to be an integer (count of active persons).
//	Produces VirtualDeviceUpdate for the corresponding Name.
type FrigateMapper struct {
	prefix string
}

// NewFrigateMapper constructs a new FrigateMapper with the given prefix (e.g. "frigate/").
func NewFrigateMapper(prefix string) *FrigateMapper {

	return &FrigateMapper{
		prefix: prefix,
	}
}

// SubscriptionTopics returns the wildcard topics needed for discovery and updates.
func (m *FrigateMapper) SubscriptionTopics() []string {
	return []string{
		m.prefix + "+/enabled/state",
		m.prefix + "+/person/active",
	}
}

// DiscoverDevicesFromMessage handles frigate/<camera>/enabled/state topics.
func (m *FrigateMapper) DiscoverDevicesFromMessage(topic string, payload []byte) ([]*VirtualDevice, error) {
	if !strings.HasPrefix(topic, m.prefix) {
		return nil, nil
	}
	suffix := strings.TrimPrefix(topic, m.prefix)
	parts := strings.Split(suffix, "/")
	// Expected: <camera>/enabled/state
	if len(parts) != 3 || parts[1] != "enabled" || parts[2] != "state" {
		return nil, nil
	}
	camera := parts[0]

	dev := &VirtualDevice{
		ID:   m.prefix + "person/" + camera,
		Type: VdevTypePerson,
		MapperData: &FrigateMapperData{
			CameraName: camera,
		},
	}

	return []*VirtualDevice{dev}, nil
}

// UpdateDevicesFromMessage handles frigate/<camera>/person/active topics (person counts).
func (m *FrigateMapper) UpdateDevicesFromMessage(topic string, payload []byte) ([]*VirtualDeviceUpdate, error) {
	if !strings.HasPrefix(topic, m.prefix) {
		return nil, nil
	}
	suffix := strings.TrimPrefix(topic, m.prefix)
	parts := strings.Split(suffix, "/")
	// Expected: <camera>/person/active
	if len(parts) != 3 || parts[1] != "person" || parts[2] != "active" {
		return nil, nil
	}
	camera := parts[0]

	// Payload expected to be ASCII integer.
	count, err := strconv.Atoi(strings.TrimSpace(string(payload)))
	if err != nil {
		// Treat malformed payload as zero but log once.
		log.Printf("[frigate] invalid person count payload for camera %s: %q (%v)", camera, string(payload), err)
		return nil, nil
	}

	update := &VirtualDeviceUpdate{
		Name:  m.prefix + "person/" + camera,
		State: count,
	}
	return []*VirtualDeviceUpdate{update}, nil
}
