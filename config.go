package main

type Config struct {
	Frigate  FrigateConfig  `yaml:"frigate"`
	MQTT     MQTTConfig     `yaml:"mqtt"`
	Rooms    []RoomConfig   `yaml:"rooms"`
	Oidc     *OidcConfig    `yaml:"oidc"`
	Database DatabaseConfig `yaml:"database"`
	Web      WebConfig      `yaml:"web"`
	SpaceAPI SpaceAPIConfig `yaml:"spaceapi"`
	Branding BrandingConfig `yaml:"branding"`
}

type BrandingConfig struct {
	PageTitle         string `yaml:"page_title" json:"page_title"`
	LogoURL           string `yaml:"logo_url" json:"logo_url"`
	LogoDarkURL       string `yaml:"logo_dark_url" json:"logo_dark_url"`
	LogoAlt           string `yaml:"logo_alt" json:"logo_alt"`
	LogoLinkURL       string `yaml:"logo_link_url" json:"logo_link_url"`
	FaviconURL        string `yaml:"favicon_url" json:"favicon_url"`
	FooterName        string `yaml:"footer_name" json:"footer_name"`
	FooterNameLinkURL string `yaml:"footer_name_link_url" json:"footer_name_link_url"`
}

type SpaceAPIConfig struct {
	Space    string                 `yaml:"space"`
	Logo     string                 `yaml:"logo"`
	Url      string                 `yaml:"url"`
	Location SpaceAPILocationConfig `yaml:"location"`
	Contact  SpaceAPIContactConfig  `yaml:"contact"`
}

type SpaceAPILocationConfig struct {
	Address  string  `yaml:"address"`
	Lat      float64 `yaml:"lat"`
	Lon      float64 `yaml:"lon"`
	Timezone string  `yaml:"timezone"`
}

type SpaceAPIContactConfig struct {
	Email    string `yaml:"email"`
	Irc      string `yaml:"irc"`
	Twitter  string `yaml:"twitter"`
	Facebook string `yaml:"facebook"`
	Phone    string `yaml:"phone"`
	Sip      string `yaml:"sip"`
	Ml       string `yaml:"ml"`
	Mastodon string `yaml:"mastodon"`
	Matrix   string `yaml:"matrix"`
	Xmpp     string `yaml:"xmpp"`
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
}

type OidcConfig struct {
	ClientID                           string   `yaml:"client_id"`
	ClientSecret                       string   `yaml:"client_secret"`
	ClientSecretFile                   string   `yaml:"client_secret_file"`
	IssuerURL                          string   `yaml:"issuer_url"`
	ExtraScopes                        []string `yaml:"extra_scopes"`
	UsernameClaim                      string   `yaml:"username_claim"`
	MembershipExpirationTimestampClaim string   `yaml:"membership_expiration_timestamp_claim"`
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

	// Prohibits controlling this device from the web page
	ProhibitControl bool `yaml:"prohibit_control"`

	// Negates the numeric value (value = -value)
	// Used for esphome power consumption sensors, which
	// have the CT transformer installed backwards
	NegateValue bool `yaml:"negate_value"`
}

type RoomConfig struct {
	ID            string          `yaml:"id"`
	LocalizedName LocalizedString `yaml:"localized_name"`

	Cameras  []string       `yaml:"cameras"`
	Entities []EntityConfig `yaml:"entities"`
}
