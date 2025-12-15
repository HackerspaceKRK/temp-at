package main

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type EntityState struct {
	ID             string          `json:"id"`
	LocalizedName  LocalizedString `json:"localized_name"`
	State          any             `json:"state"`
	Type           string          `json:"type"`
	Representation string          `json:"representation"`
}

type RoomState struct {
	ID            string          `json:"id"`
	LocalizedName LocalizedString `json:"localized_name"`
	// PeopleCount is the number of people in the room
	// (it is calculated by taking the maximum as reported by each camera)
	PeopleCount int `json:"people_count"`
	// LatestPersonDetectedAt is the timestamp (Unix milliseconds) when a person was last
	// detected in this room. Only set when PeopleCount is 0.
	LatestPersonDetectedAt *time.Time    `json:"latest_person_detected_at"`
	Entities               []EntityState `json:"entities"`
}

func buildRoomState(id string) *RoomState {
	for _, r := range ConfigInstance.Rooms {
		if r.ID == id {
			rs := &RoomState{
				ID:            id,
				LocalizedName: r.LocalizedName,
				Entities:      []EntityState{},
			}

			// Use VirtualDevices from mqtt to create entities
			virtDevices := vdevManager.Devices()
			var personDevices []string // Track person device IDs in this room

			for _, e := range r.Entities {
				es := EntityState{
					ID:             e.ID,
					Representation: e.Representation,
					LocalizedName:  e.LocalizedName,
				}

				for _, v := range virtDevices {
					if v.ID == e.ID {
						// Use the maximum people count reported by any camera in the room
						if v.Type == VdevTypePerson && v.State != nil {
							personDevices = append(personDevices, v.ID)
							intVal, ok := v.State.(int)
							if ok && intVal > rs.PeopleCount {
								rs.PeopleCount = intVal
							}
						}
						es.State = v.State
						es.Type = string(v.Type)
						break
					}

				}

				rs.Entities = append(rs.Entities, es)
			}

			// If room is empty, find the latest person detection time
			if rs.PeopleCount == 0 && len(personDevices) > 0 && vdevHistoryRepo != nil {
				log.Printf("Room %s is empty, finding latest person detection time", id)
				var latestTimestamp *int64
				for _, deviceID := range personDevices {
					ts, err := vdevHistoryRepo.GetLatestPersonDetectionTime(deviceID)
					if err != nil {
						log.Printf("Failed to get latest person detection time for %s: %v", deviceID, err)
						continue
					}
					if ts != nil && (latestTimestamp == nil || *ts > *latestTimestamp) {
						latestTimestamp = ts
					}
				}
				if latestTimestamp != nil {
					parsed := time.Unix((*latestTimestamp)/1000, 0)
					log.Printf("Latest person detection time for room %s: %v, %v", id, latestTimestamp, parsed)
					rs.LatestPersonDetectedAt = &parsed
				}
			}

			return rs
		}
	}
	return &RoomState{}
}

func buildRoomStates() []*RoomState {
	states := []*RoomState{}

	for _, room := range ConfigInstance.Rooms {
		states = append(states, buildRoomState(room.ID))
	}

	return states
}

func handleGetRoomStates(c *fiber.Ctx) error {
	states := buildRoomStates()
	return c.JSON(states)
}

func handleVirtualDeviceStateUpdate(vdev *VirtualDevice) {
	devName := vdev.ID

	var room *RoomConfig
	for _, r := range ConfigInstance.Rooms {
		for _, ent := range r.Entities {
			if ent.ID == devName {
				room = &r
				break
			}
		}
	}

	if room != nil {
		socketChansMutex.Lock()
		defer socketChansMutex.Unlock()
		for _, ch := range socketChans {

			select {
			case ch <- buildRoomState(room.ID):
			default:
			}
		}
	}
}

var socketChans = []chan *RoomState{}
var socketChansMutex = sync.Mutex{}

func handleLiveWs(c *websocket.Conn) {

	// First of all send all room states as an initial message
	for _, room := range ConfigInstance.Rooms {
		rs := buildRoomState(room.ID)
		err := c.WriteJSON(rs)
		if err != nil {
			log.Printf("Failed to send initial room state to WS: %v", err)
			return
		}
	}

	recvChan := make(chan *RoomState, 20)
	socketChansMutex.Lock()

	socketChans = append(socketChans, recvChan)
	socketChansMutex.Unlock()

	defer func() {
		socketChansMutex.Lock()
		defer socketChansMutex.Unlock()
		for i, ch := range socketChans {
			if ch == recvChan {
				socketChans = append(socketChans[:i], socketChans[i+1:]...)
				break
			}
		}
	}()
	for r := range recvChan {
		err := c.WriteJSON(r)
		if err != nil {
			log.Printf("Failed to send updated room state to WS: %v", err)
			break
		}
	}
}
