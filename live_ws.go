package main

import (
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
	Name     string
	Entities []EntityState
}

func buildRoomState(name string) RoomState {
	for _, r := range ConfigInstance.Rooms {
		if r.Name == name {
			entities := []EntityState{}
			virtDevices := mqttAdapter.VirtualDevices()

			for _, e := range r.Entities {
				es := EntityState{
					Name:           e.Name,
					Representation: e.Representation,
				}

				for _, v := range virtDevices {
					if v.Name == e.Name {
						es.State = v.State
						es.Type = v.Type
						break
					}
				}

				entities = append(entities, es)
			}

			return RoomState{
				Name:     name,
				Entities: entities,
			}
		}
	}
	return RoomState{}
}

func buildRoomStates() []RoomState {
	states := []RoomState{}

	for _, room := range ConfigInstance.Rooms {
		states = append(states, buildRoomState(room.Name))
	}

	return states
}

func handleGetRoomStates(c *fiber.Ctx) error {
	states := buildRoomStates()
	return c.JSON(states)
}

func handleLiveWs(c *websocket.Conn) {

}
