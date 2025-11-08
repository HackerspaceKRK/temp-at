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

	// Friendly name that is used in the UI
	FriendlyName string `yaml:"friendly_name"`

	// How the device should be represented in the UI (light, fan, etc.)
	Representation string `yaml:"representation"`
}

type RoomConfig struct {
	Name string `yaml:"name"`

	Cameras  []string       `yaml:"cameras"`
	Entities []EntityConfig `yaml:"entities"`
}
