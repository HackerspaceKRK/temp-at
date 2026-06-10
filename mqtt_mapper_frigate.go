package main

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// onConnectInterval is how often we re-publish frigate/onConnect to force Frigate
// to re-emit a retained camera_activity message.
const onConnectInterval = 5 * time.Minute

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
//	    ID:        frigate/person/<camera>
//	    Type:      person
//
// Updates:
//
//	Subscribes to:   frigate/camera_activity
//	A single JSON message reports the activity of every camera at once. For each
//	camera we count the objects labelled "person" (both stationary and moving) and
//	emit a VirtualDeviceUpdate with that count.
//
// On connect (and periodically every onConnectInterval) the mapper publishes
// frigate/onConnect = "1" to force Frigate to refresh / re-emit camera_activity.
type FrigateMapper struct {
	prefix string

	// tickerOnce guards starting the periodic onConnect publisher exactly once.
	tickerOnce sync.Once
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
		m.prefix + "camera_activity",
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

// cameraActivity mirrors the subset of the frigate/camera_activity payload we need.
type cameraActivity struct {
	Objects []struct {
		Label string `json:"label"`
	} `json:"objects"`
}

// UpdateDevicesFromMessage handles the frigate/camera_activity topic.
//
// The payload is a JSON object keyed by camera name; for each camera we count the
// objects labelled "person" (regardless of whether they are stationary or moving).
func (m *FrigateMapper) UpdateDevicesFromMessage(topic string, payload []byte) ([]*VirtualDeviceUpdate, error) {
	if topic != m.prefix+"camera_activity" {
		return nil, nil
	}

	var activity map[string]cameraActivity
	if err := json.Unmarshal(payload, &activity); err != nil {
		log.Printf("[frigate] invalid camera_activity payload: %v", err)
		return nil, nil
	}

	updates := make([]*VirtualDeviceUpdate, 0, len(activity))
	for camera, data := range activity {
		count := 0
		for _, obj := range data.Objects {
			if obj.Label == "person" {
				count++
			}
		}
		updates = append(updates, &VirtualDeviceUpdate{
			Name:  m.prefix + "person/" + camera,
			State: count,
		})
	}
	return updates, nil
}

// OnConnect publishes frigate/onConnect immediately and starts a periodic publisher
// (once) so Frigate keeps re-emitting camera_activity. It satisfies the optional
// MapperConnectHook interface invoked by MQTTAdapter on every (re)connect.
func (m *FrigateMapper) OnConnect(client mqtt.Client) {
	m.publishOnConnect(client)

	m.tickerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(onConnectInterval)
			defer ticker.Stop()
			for range ticker.C {
				m.publishOnConnect(client)
			}
		}()
	})
}

// publishOnConnect publishes frigate/onConnect = "1".
func (m *FrigateMapper) publishOnConnect(client mqtt.Client) {
	token := client.Publish(m.prefix+"onConnect", 0, false, "1")
	token.Wait()
	if err := token.Error(); err != nil {
		log.Printf("[frigate] failed to publish onConnect: %v", err)
	}
}

// Control is a no-op for Frigate devices.
func (m *FrigateMapper) Control(vdev *VirtualDevice, state any, client mqtt.Client) error {
	return nil
}
