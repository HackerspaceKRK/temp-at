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

type RoomConfig struct {
	Name    string   `yaml:"name"`
	Cameras []string `yaml:"cameras"`
	Relays  []string `yaml:"relays"`
	Sensors []string `yaml:"sensors"`
}
