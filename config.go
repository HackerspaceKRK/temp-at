package main

type Config struct {
	Frigate FrigateConfig `yaml:"frigate"`
	MQTT    MQTTConfig    `yaml:"mqtt"`
	Rooms   []RoomConfig  `yaml:"rooms"`
}

// LocalizedString represents a string localized into multiple languages.
// The keys are language codes (e.g. "en", "de"), and the values are the localized strings.
type LocalizedString map[string]string

type FrigateConfig struct {
	Url string `yaml:"url"`
}

type MQTTConfig struct {
	Broker   string `yaml:"broker"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type EntityConfig struct {
	// ID that identifies this entity/device. Same value as .ID in VirtualDevice
	ID string `yaml:"id"`

	LocalizedName LocalizedString `yaml:"localized_name"`

	// How the device should be represented in the UI (light, fan, etc.)
	Representation string `yaml:"representation"`
}

type RoomConfig struct {
	ID            string          `yaml:"id"`
	LocalizedName LocalizedString `yaml:"localized_name"`

	Cameras  []string       `yaml:"cameras"`
	Entities []EntityConfig `yaml:"entities"`
}
