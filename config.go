package main

type Config struct {
	Frigate     FrigateConfig     `yaml:"frigate"`
	MQTT        MQTTConfig        `yaml:"mqtt"`
	Rooms       []RoomConfig      `yaml:"rooms"`
	Oidc        *OidcConfig       `yaml:"oidc"`
	Database    DatabaseConfig    `yaml:"database"`
	Web         WebConfig         `yaml:"web"`
	SpaceAPI    SpaceAPIConfig    `yaml:"spaceapi"`
	Branding    BrandingConfig    `yaml:"branding"`
	Tablet      TabletConfig      `yaml:"tablet"`
	Phabricator PhabricatorConfig `yaml:"phabricator"`
	// Dhcp is optional. When nil, the DHCP lease tracking feature is disabled
	// and the /api/v1/dhcp/leases endpoint returns 503.
	Dhcp *DhcpConfig `yaml:"dhcp"`
	// BambuPrinters optionally configures Bambu Labs 3D printers to monitor. Each
	// printer is polled over its own TLS MQTT broker and exposed as a "printer"
	// virtual device (whose state is never persisted to the database).
	BambuPrinters []BambuPrinterConfig `yaml:"bambu_printers"`
}

// BambuPrinterConfig describes a single Bambu Labs printer reachable over its
// local MQTT interface (TLS, self-signed cert). It is exposed as a virtual
// device whose ID is referenced from a room's entities.
type BambuPrinterConfig struct {
	ID           string `yaml:"id"`            // vdev id, also used in room entities
	SerialNumber string `yaml:"serial_number"` // MQTT topic: device/<serial>/report
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`     // default 8883
	Username     string `yaml:"username"` // typically "bblp"
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
	// InsecureSkipVerify accepts the printer's self-signed TLS certificate
	// (equivalent to mosquitto_sub --insecure). Usually required.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// DhcpConfig configures the DHCP lease tracking feature: a background scraper
// that polls a router's DHCP server, enriches leases with switch-port and WiFi
// info, and exposes them on an authenticated, CIDR-filtered API.
type DhcpConfig struct {
	// ScrapeInterval is a Go duration (e.g. "1m") between lease scrapes. Default "1m".
	ScrapeInterval string `yaml:"scrape_interval"`
	// OfflineThreshold is a Go duration (e.g. "3m"). A device absent from the
	// lease table for longer than this is considered offline, and its next
	// reappearance starts a new continuous lease period. Default "3m".
	OfflineThreshold string `yaml:"offline_threshold"`
	// PruneAfter optionally deletes lease rows not seen for this long (Go
	// duration, e.g. "2160h"). Empty disables pruning.
	PruneAfter string `yaml:"prune_after"`

	// Router is the DHCP lease source.
	Router DhcpRouterConfig `yaml:"router"`
	// WiredSources enrich leases with switch port info (MAC -> switch/port).
	WiredSources []DhcpWiredSourceConfig `yaml:"wired_sources"`
	// WifiSources enrich leases with WiFi info (MAC -> AP/SSID/RSSI).
	WifiSources []DhcpWifiSourceConfig `yaml:"wifi_sources"`
	// Access controls which OIDC groups may see devices in which subnets.
	Access DhcpAccessConfig `yaml:"access"`
}

// DhcpRouterConfig points at the device whose DHCP server is scraped for leases.
type DhcpRouterConfig struct {
	Kind         string `yaml:"kind"`    // e.g. "mikrotik"
	Address      string `yaml:"address"` // host:port, e.g. "10.12.20.1:8728"
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
	UseTLS       bool   `yaml:"use_tls"` // dial the TLS API (port 8729 on MikroTik)
}

// DhcpWiredSourceConfig points at a switch whose MAC-to-port table is read to
// determine which physical port a wired device is connected to.
type DhcpWiredSourceConfig struct {
	Kind         string `yaml:"kind"` // e.g. "mikrotik_bridge_host"
	Name         string `yaml:"name"` // display name shown in the UI as the switch name
	Address      string `yaml:"address"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
	UseTLS       bool   `yaml:"use_tls"`
	// IgnoreInterfaces lists port/interface names to skip (uplink/trunk ports
	// that carry many MACs and would otherwise mislabel devices).
	IgnoreInterfaces []string `yaml:"ignore_interfaces"`
}

// DhcpWifiSourceConfig points at a wireless controller that reports which AP a
// client is associated with, plus SSID and signal strength.
type DhcpWifiSourceConfig struct {
	Kind               string `yaml:"kind"` // e.g. "unifi"
	Name               string `yaml:"name"`
	URL                string `yaml:"url"`  // e.g. "https://10.12.20.5:8443"
	Site               string `yaml:"site"` // controller site id, e.g. "default"
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	PasswordFile       string `yaml:"password_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"` // accept self-signed TLS certs
}

// DhcpAccessConfig maps OIDC groups to the subnets (CIDRs) whose devices that
// group may see. Authenticated users whose groups match no entry see
// DefaultCidrs. The endpoint is never accessible anonymously.
type DhcpAccessConfig struct {
	// GroupCidrs maps an OIDC group name to a list of CIDRs it may view.
	GroupCidrs map[string][]string `yaml:"group_cidrs"`
	// DefaultCidrs are visible to authenticated users with no matching group.
	DefaultCidrs []string `yaml:"default_cidrs"`
}

// TabletConfig controls the wall-mounted tablet/kiosk mode.
type TabletConfig struct {
	// TrustedSubnets is a list of CIDRs. A tablet connecting from one of these
	// subnets is granted a long-lived session that may control devices.
	TrustedSubnets []string `yaml:"trusted_subnets"`
}

// PhabricatorConfig points at the Phabricator/Phorge instance whose Calendar
// is shown as room reservations on the tablet.
type PhabricatorConfig struct {
	URL          string `yaml:"url"`
	APIToken     string `yaml:"api_token"`
	APITokenFile string `yaml:"api_token_file"`
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
	ListenAddress   string   `yaml:"listen_address"`
	PublicURL       string   `yaml:"public_url"`
	SpaceapiDomains []string `yaml:"spaceapi_domains"`
	// TrustedProxies is a list of IPs/CIDRs of reverse proxies (e.g. Traefik)
	// allowed to set the X-Forwarded-For header. When set, the real client IP
	// is taken from that header instead of the immediate peer.
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type OidcConfig struct {
	ClientID                           string   `yaml:"client_id"`
	ClientSecret                       string   `yaml:"client_secret"`
	ClientSecretFile                   string   `yaml:"client_secret_file"`
	IssuerURL                          string   `yaml:"issuer_url"`
	ExtraScopes                        []string `yaml:"extra_scopes"`
	UsernameClaim                      string   `yaml:"username_claim"`
	MembershipExpirationTimestampClaim string   `yaml:"membership_expiration_timestamp_claim"`
	GroupsClaim                        string   `yaml:"groups_claim"`
	DebugAccessGroups                  []string `yaml:"debug_access_groups"`
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
	ID                        string          `yaml:"id"`
	LocalizedName             LocalizedString `yaml:"localized_name"`
	ExcludeFromEntranceTablet bool            `yaml:"exclude_from_entrance_tablet"`

	// VoipPhoneNumber is the SIP extension/number to dial to reach this room.
	// Optional; rooms without it are not shown on the tablet phone page.
	VoipPhoneNumber string `yaml:"voip_phone_number"`

	Cameras  []string       `yaml:"cameras"`
	Entities []EntityConfig `yaml:"entities"`
}
