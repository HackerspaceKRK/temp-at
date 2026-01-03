package main

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func handleSpaceAPI(c *fiber.Ctx) error {
	cfg := MustLoadConfig()
	devices := vdevManager.Devices()
	deviceMap := make(map[string]*VirtualDevice)
	for _, dev := range devices {
		deviceMap[dev.ID] = dev
	}

	api := A15Json{
		ApiCompatibility: []string{"15"},
		Space:            cfg.SpaceAPI.Space,
		Logo:             cfg.SpaceAPI.Logo,
		Url:              cfg.SpaceAPI.Url,
		Contact: A15JsonContact{
			Email:    stringPtr(cfg.SpaceAPI.Contact.Email),
			Irc:      stringPtr(cfg.SpaceAPI.Contact.Irc),
			Twitter:  stringPtr(cfg.SpaceAPI.Contact.Twitter),
			Facebook: stringPtr(cfg.SpaceAPI.Contact.Facebook),
			Phone:    stringPtr(cfg.SpaceAPI.Contact.Phone),
			Sip:      stringPtr(cfg.SpaceAPI.Contact.Sip),
			Ml:       stringPtr(cfg.SpaceAPI.Contact.Ml),
			Mastodon: stringPtr(cfg.SpaceAPI.Contact.Mastodon),
			Matrix:   stringPtr(cfg.SpaceAPI.Contact.Matrix),
			Xmpp:     stringPtr(cfg.SpaceAPI.Contact.Xmpp),
		},
		Location: &A15JsonLocation{
			Address:  stringPtr(cfg.SpaceAPI.Location.Address),
			Lat:      &cfg.SpaceAPI.Location.Lat,
			Lon:      &cfg.SpaceAPI.Location.Lon,
			Timezone: stringPtr(cfg.SpaceAPI.Location.Timezone),
		},
		Sensors: &A15JsonSensors{},
		State: &A15JsonState{
			Open: boolPtr(false),
		},
	}

	for _, room := range cfg.Rooms {
		roomLabel := room.ID
		if name, ok := room.LocalizedName["en"]; ok && name != "" {
			roomLabel = name
		}

		roomPeopleCount := 0.0
		hasPeopleSensor := false

		for _, entity := range room.Entities {
			dev, ok := deviceMap[entity.ID]
			if !ok {
				continue
			}

			entityName := entity.ID
			if name, ok := entity.LocalizedName["en"]; ok && name != "" {
				entityName = name
			}

			val, isValid := toFloat64Internal(dev.State)
			if !isValid {
				continue
			}

			switch dev.Type {
			case VdevTypeTemperature:
				api.Sensors.Temperature = append(api.Sensors.Temperature, A15JsonSensorsTemperatureElem{
					Location: roomLabel,
					Name:     &entityName,
					Unit:     A15JsonSensorsTemperatureElemUnitC,
					Value:    val,
				})
			case VdevTypeHumidity:
				api.Sensors.Humidity = append(api.Sensors.Humidity, A15JsonSensorsHumidityElem{
					Location: roomLabel,
					Name:     &entityName,
					Unit:     A15JsonSensorsHumidityElemUnitUndefined,
					Value:    val,
				})
			case VdevTypePerson:
				roomPeopleCount += val
				hasPeopleSensor = true
			}
		}

		if hasPeopleSensor {
			api.Sensors.PeopleNowPresent = append(api.Sensors.PeopleNowPresent, A15JsonSensorsPeopleNowPresentElem{
				Location: &roomLabel,
				Value:    roomPeopleCount,
			})
		}
	}

	if len(api.Sensors.PeopleNowPresent) > 0 {
		totalPeople := 0.0
		for _, p := range api.Sensors.PeopleNowPresent {
			totalPeople += p.Value
		}
		api.Sensors.PeopleNowPresent = append(api.Sensors.PeopleNowPresent, A15JsonSensorsPeopleNowPresentElem{
			Name:  stringPtr("total"),
			Value: totalPeople,
		})
	}

	return c.JSON(api)
}

func toFloat64Internal(v any) (float64, bool) {
	switch val := v.(type) {
	case bool:
		if val {
			return 1.0, true
		}
		return 0.0, true
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		lower := strings.ToLower(val)
		if lower == "on" || lower == "true" {
			return 1.0, true
		} else if lower == "off" || lower == "false" {
			return 0.0, true
		} else {
			if parsed, err := strconv.ParseFloat(val, 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
