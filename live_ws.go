package main

import (
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type EntityState struct {
	Name           string `json:"name"`
	State          any    `json:"state"`
	Type           string `json:"type"`
	Representation string `json:"representation"`
}

type RoomState struct {
	Name string `json:"name"`
	// PeopleCount is the number of people in the room
	// (it is calculated by taking the maximum as reported by each camera)
	PeopleCount int           `json:"people_count"`
	Entities    []EntityState `json:"entities"`
}

func buildRoomState(name string) *RoomState {
	for _, r := range ConfigInstance.Rooms {
		if r.Name == name {
			rs := &RoomState{
				Name:     name,
				Entities: []EntityState{},
			}
			virtDevices := mqttAdapter.VirtualDevices()

			for _, e := range r.Entities {
				es := EntityState{
					Name:           e.Name,
					Representation: e.Representation,
				}

				for _, v := range virtDevices {
					if v.Name == e.Name {
						if v.Type == "person" && v.State != nil {
							intVal, ok := v.State.(int)
							if ok && intVal > rs.PeopleCount {
								rs.PeopleCount = intVal
							}
						}
						es.State = v.State
						es.Type = v.Type
						break
					}

				}

				rs.Entities = append(rs.Entities, es)
			}

			return rs
		}
	}
	return &RoomState{}
}

func buildRoomStates() []*RoomState {
	states := []*RoomState{}

	for _, room := range ConfigInstance.Rooms {
		states = append(states, buildRoomState(room.Name))
	}

	return states
}

func handleGetRoomStates(c *fiber.Ctx) error {
	states := buildRoomStates()
	return c.JSON(states)
}

func handleVirtualDeviceStateUpdate(devName string) {

	var room *RoomConfig
	for _, r := range ConfigInstance.Rooms {
		for _, ent := range r.Entities {
			if ent.Name == devName {
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
			case ch <- buildRoomState(room.Name):
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
		rs := buildRoomState(room.Name)
		log.Printf("BEFORE WRITE JSON")
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
		log.Printf("Received updated room state from recvChan: %s", r.Name)
		err := c.WriteJSON(r)
		if err != nil {
			log.Printf("Failed to send updated room state to WS: %v", err)
			break
		}
	}
}
