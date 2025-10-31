package main

type Config struct {
	Frigate FrigateConfig `yaml:"frigate"`
	MQTT    MQTTConfig    `yaml:"mqtt"`
	Rooms   []RoomConfig  `yaml:"rooms"`
}

type FrigateConfig struct {
	Url string `yaml:"url"`
}

type MQTTConfig struct {
	Broker   string `yaml:"broker"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type EntityConfig struct {
	// Name that identifies this entiti/device. Same value as .Name in VirtualDevice
	Name string `yaml:"name"`

	// How the device should be represented in the UI (light, fan, etc.)
	Representation string `yaml:"representation"`
}

type RoomConfig struct {
	Name     string         `yaml:"name"`
	Entities []EntityConfig `yaml:"entities"`
}
