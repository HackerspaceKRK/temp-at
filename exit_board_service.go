package main

import (
	"log"
	"strconv"
	"strings"
)

// Entity representation values recognised by the exit board.
const (
	exitBoardReprLight  = "light"
	exitBoardReprWindow = "contact"
)

// ExitBoardService publishes a per-room status code to MQTT so an exit panel can
// show whether each room is safe to leave (lights off, windows closed).
//
// For every room containing at least one light/window entity it publishes to
// <MQTTPrefix>/<room_id>:
//   - 0: all windows closed and all lights off
//   - 1: all windows closed, at least one light on
//   - 2: at least one window open (takes priority over lights)
type ExitBoardService struct {
	cfg  *ExitBoardConfig
	vdev *VdevManager
	mqtt *MQTTAdapter

	// rooms are the configured rooms containing at least one light or window
	// entity (the only ones worth publishing).
	rooms []RoomConfig
}

// NewExitBoardService creates the service, collecting the rooms that have at
// least one light or window entity.
func NewExitBoardService(cfg *Config, vdev *VdevManager, mqtt *MQTTAdapter) *ExitBoardService {
	s := &ExitBoardService{
		cfg:  cfg.ExitBoard,
		vdev: vdev,
		mqtt: mqtt,
	}
	for _, room := range cfg.Rooms {
		if roomHasExitBoardEntities(room) {
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

// publishRoom computes and publishes the current status code for a room.
func (s *ExitBoardService) publishRoom(room RoomConfig) {
	code := s.computeCode(room)
	topic := s.cfg.MQTTPrefix + "/" + room.ID
	if err := s.mqtt.Publish(topic, []byte(strconv.Itoa(code)), true); err != nil {
		log.Printf("[exit_board] failed to publish %s: %v", topic, err)
	}
}

// computeCode derives the room status code from its light and window entities.
func (s *ExitBoardService) computeCode(room RoomConfig) int {
	// Index current device states by ID for O(1) lookup.
	devices := s.vdev.Devices()
	stateByID := make(map[string]any, len(devices))
	for _, d := range devices {
		stateByID[d.ID] = d.State
	}

	anyLightOn := false
	for _, e := range room.Entities {
		state := stateByID[e.ID]
		switch e.Representation {
		case exitBoardReprWindow:
			if isWindowOpen(state) {
				return 2 // window open takes priority over everything
			}
		case exitBoardReprLight:
			if isLightOn(state) {
				anyLightOn = true
			}
		}
	}
	if anyLightOn {
		return 1
	}
	return 0
}

// roomHasExitBoardEntities reports whether a room has any light or window entity.
func roomHasExitBoardEntities(room RoomConfig) bool {
	for _, e := range room.Entities {
		if e.Representation == exitBoardReprLight || e.Representation == exitBoardReprWindow {
			return true
		}
	}
	return false
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
