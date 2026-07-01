package main

import (
	"log"
	"strconv"
	"strings"
)

// exitBoardReprLight is the entity representation treated as a light by the exit
// board. Windows are matched by virtual-device type (any contact sensor),
// regardless of representation.
const exitBoardReprLight = "light"

// ExitBoardService publishes a per-room status code to MQTT so an exit panel can
// show whether each room is safe to leave (lights off, windows closed).
//
// For every room containing at least one light entity or contact sensor it
// publishes to <MQTTPrefix>/<room_id>:
//   - 0: all windows closed and all lights off
//   - 1: all windows closed, at least one light on
//   - 2: at least one window open (takes priority over lights)
type ExitBoardService struct {
	cfg  *ExitBoardConfig
	vdev *VdevManager
	mqtt *MQTTAdapter

	// rooms are the configured rooms that have any entities. Whether a room is
	// actually relevant (has a light or contact sensor) is decided dynamically at
	// publish time, because contact vdevs are discovered from MQTT after startup.
	rooms []RoomConfig
}

// NewExitBoardService creates the service, collecting the rooms that have any
// entities.
func NewExitBoardService(cfg *Config, vdev *VdevManager, mqtt *MQTTAdapter) *ExitBoardService {
	s := &ExitBoardService{
		cfg:  cfg.ExitBoard,
		vdev: vdev,
		mqtt: mqtt,
	}
	for _, room := range cfg.Rooms {
		if len(room.Entities) > 0 {
			s.rooms = append(s.rooms, room)
		}
	}
	return s
}

// Start publishes the current status of every relevant room once (seeding the
// retained topics from restored state) and registers a callback so subsequent
// device changes republish the affected room.
func (s *ExitBoardService) Start() {
	for _, room := range s.rooms {
		s.publishRoom(room)
	}
	s.vdev.OnVirtualDeviceUpdated = append(s.vdev.OnVirtualDeviceUpdated, s.onDeviceUpdate)
}

// onDeviceUpdate republishes any room that contains the changed device.
func (s *ExitBoardService) onDeviceUpdate(v *VirtualDevice) {
	if v == nil {
		return
	}
	for _, room := range s.rooms {
		for _, e := range room.Entities {
			if e.ID == v.ID {
				s.publishRoom(room)
				break
			}
		}
	}
}

// publishRoom computes and publishes the current status code for a room. Rooms
// with neither a light nor a contact sensor are skipped.
func (s *ExitBoardService) publishRoom(room RoomConfig) {
	code, relevant := s.computeCode(room)
	if !relevant {
		return
	}
	topic := s.cfg.MQTTPrefix + "/" + room.ID
	if err := s.mqtt.Publish(topic, []byte(strconv.Itoa(code)), true); err != nil {
		log.Printf("[exit_board] failed to publish %s: %v", topic, err)
	}
}

// computeCode derives the room status code from its lights and contact sensors.
// relevant is false when the room has neither, in which case nothing is published.
func (s *ExitBoardService) computeCode(room RoomConfig) (code int, relevant bool) {
	// Index current devices by ID for O(1) lookup.
	devices := s.vdev.Devices()
	byID := make(map[string]*VirtualDevice, len(devices))
	for _, d := range devices {
		byID[d.ID] = d
	}

	windowOpen := false
	anyLightOn := false
	for _, e := range room.Entities {
		dev := byID[e.ID]

		// Windows: any contact sensor, regardless of representation.
		if dev != nil && dev.Type == VdevTypeContact {
			relevant = true
			if isWindowOpen(dev.State) {
				windowOpen = true
			}
			continue
		}

		// Lights: entities represented as lights.
		if e.Representation == exitBoardReprLight {
			relevant = true
			var state any
			if dev != nil {
				state = dev.State
			}
			if isLightOn(state) {
				anyLightOn = true
			}
		}
	}

	switch {
	case windowOpen:
		return 2, relevant // window open takes priority over lights
	case anyLightOn:
		return 1, relevant
	default:
		return 0, relevant
	}
}

// isWindowOpen interprets a contact-sensor state. Zigbee2MQTT reports
// contact:true when the window is closed, so a bool false means open. Unknown
// (nil / non-bool) state is treated as closed.
func isWindowOpen(state any) bool {
	b, ok := state.(bool)
	return ok && !b
}

// isLightOn interprets a relay state. Relays report the string "ON"/"OFF".
// Unknown (nil / non-string) state is treated as off.
func isLightOn(state any) bool {
	s, ok := state.(string)
	return ok && strings.EqualFold(s, "ON")
}
