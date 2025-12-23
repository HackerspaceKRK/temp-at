package main

type Config struct {
	Frigate  FrigateConfig  `yaml:"frigate"`
	MQTT     MQTTConfig     `yaml:"mqtt"`
	Rooms    []RoomConfig   `yaml:"rooms"`
	Oidc     *OidcConfig    `yaml:"oidc"`
	Database DatabaseConfig `yaml:"database"`
	Web      WebConfig      `yaml:"web"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"` // SQLite file path
}

// LocalizedString represents a string localized into multiple languages.
// The keys are language codes (e.g. "en", "de"), and the values are the localized strings.
type LocalizedString map[string]string

type WebConfig struct {
	ListenAddress string `yaml:"listen_address"`
	PublicURL     string `yaml:"public_url"`
	JWTSecret     string `yaml:"jwt_secret"`
	JWTSecretFile string `yaml:"jwt_secret_file"`
}

type OidcConfig struct {
	ClientID         string `yaml:"client_id"`
	ClientSecret     string `yaml:"client_secret"`
	ClientSecretFile string `yaml:"client_secret_file"`
	IssuerURL        string `yaml:"issuer_url"`
}

type FrigateConfig struct {
	Url string `yaml:"url"`
}

type MQTTConfig struct {
	Broker       string `yaml:"broker"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
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
