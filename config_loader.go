package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"strings"

	"github.com/goccy/go-yaml"
)

// Config is defined in config.go

// Global immutable configuration instance.
// Loaded once by MustLoadConfig() at program start.
var ConfigInstance *Config

// MustLoadConfig loads the configuration from the first existing candidate path
// and stores it in ConfigInstance. It fatals if no valid config is found.
// If CONFIG_PATH env var is set, it is tried first.
func MustLoadConfig() *Config {
	if ConfigInstance != nil {
		return ConfigInstance
	}

	candidates := []string{
		os.Getenv("CONFIG_PATH"),
		"at2.yaml",
		"./at2.yaml",
		"/etc/at2.yaml",
	}

	var tried []string
	for _, path := range candidates {
		if path == "" {
			continue
		}
		tried = append(tried, path)

		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Fatalf("Failed to parse YAML in %s: %v", path, err)
		}

		validateConfig(&cfg, path)
		ConfigInstance = &cfg
		log.Printf("Loaded config from %s", path)
		return ConfigInstance
	}

	log.Fatalf("No configuration file found. Tried: %v", tried)
	return nil
}

// GetConfig returns the loaded configuration (nil if not yet loaded).
func GetConfig() *Config {
	return ConfigInstance
}

// Basic validation & warnings.
func validateConfig(cfg *Config, path string) {
	loadSecret(&cfg.MQTT.Password, cfg.MQTT.PasswordFile)
	loadSecret(&cfg.Oidc.ClientSecret, cfg.Oidc.ClientSecretFile)
	loadSecret(&cfg.Oidc.ClientSecret, cfg.Oidc.ClientSecretFile)
	loadSecret(&cfg.Phabricator.APIToken, cfg.Phabricator.APITokenFile)
	validateDhcpConfig(cfg, path)

	for i := range cfg.BambuPrinters {
		loadSecret(&cfg.BambuPrinters[i].Password, cfg.BambuPrinters[i].PasswordFile)
		if cfg.BambuPrinters[i].ID == "" {
			log.Printf("warning: bambu_printers[%d] has empty id in %s", i, path)
		}
		if cfg.BambuPrinters[i].SerialNumber == "" {
			log.Printf("warning: bambu_printers[%d] (%s) has empty serial_number in %s", i, cfg.BambuPrinters[i].ID, path)
		}
	}

	if cfg.Frigate.Url == "" {
		log.Printf("warning: frigate.url is empty in %s", path)
	}
	if cfg.MQTT.Broker == "" {
		log.Printf("warning: mqtt.broker is empty in %s", path)
	}
	nameSeen := map[string]struct{}{}
	for i, room := range cfg.Rooms {
		if room.ID == "" {
			log.Printf("warning: rooms[%d] has empty ID in %s", i, path)
		} else if _, dup := nameSeen[room.ID]; dup {
			log.Fatalf("error: duplicate room ID %q in %s", room.ID, path)
		} else {
			nameSeen[room.ID] = struct{}{}
		}
	}
	if len(cfg.Rooms) == 0 {
		log.Printf("warning: No rooms defined in %s", path)
	}
}

// validateDhcpConfig loads DHCP secrets from their _file variants and fails
// fast on malformed durations, CIDRs, or unknown source kinds.
func validateDhcpConfig(cfg *Config, path string) {
	d := cfg.Dhcp
	if d == nil {
		return
	}

	loadSecret(&d.Router.Password, d.Router.PasswordFile)
	for i := range d.WiredSources {
		loadSecret(&d.WiredSources[i].Password, d.WiredSources[i].PasswordFile)
	}
	for i := range d.WifiSources {
		loadSecret(&d.WifiSources[i].Password, d.WifiSources[i].PasswordFile)
	}

	mustDuration := func(field, val string) {
		if val == "" {
			return
		}
		if _, err := time.ParseDuration(val); err != nil {
			log.Fatalf("error: dhcp.%s is not a valid duration (%q) in %s: %v", field, val, path, err)
		}
	}
	mustDuration("scrape_interval", d.ScrapeInterval)
	mustDuration("offline_threshold", d.OfflineThreshold)
	mustDuration("prune_after", d.PruneAfter)

	mustCidrs := func(field string, cidrs []string) {
		for _, c := range cidrs {
			if _, _, err := net.ParseCIDR(c); err != nil {
				log.Fatalf("error: dhcp.%s contains invalid CIDR %q in %s: %v", field, c, path, err)
			}
		}
	}
	mustCidrs("access.default_cidrs", d.Access.DefaultCidrs)
	for group, cidrs := range d.Access.GroupCidrs {
		mustCidrs(fmt.Sprintf("access.group_cidrs[%s]", group), cidrs)
	}

	if d.Router.Kind == "" {
		log.Printf("warning: dhcp.router.kind is empty in %s; lease scraping disabled", path)
	}
}

func loadSecret(target *string, file string) {
	if *target == "" && file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("warning: failed to read secret from file %s: %v", file, err)
			return
		}
		*target = strings.TrimSpace(string(data))
	}
}
