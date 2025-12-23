package main

import (
	"log"
	"os"

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
	loadSecret(&cfg.Web.JWTSecret, cfg.Web.JWTSecretFile)

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
	// Example of a hard check (uncomment if desired):
	// if len(cfg.Rooms) == 0 {
	//	log.Fatalf("No rooms defined in %s", path)
	// }
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
