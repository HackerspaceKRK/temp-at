package main

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusCollector collects metrics from VirtualDevices
type PrometheusCollector struct {
	vdevManager *VdevManager
	config      *Config

	// Cache room lookup
	deviceRoomMap map[string]string // deviceID -> roomID
}

func NewPrometheusCollector(vm *VdevManager, cfg *Config) *PrometheusCollector {
	pc := &PrometheusCollector{
		vdevManager:   vm,
		config:        cfg,
		deviceRoomMap: make(map[string]string),
	}

	// Pre-build device -> room map
	for _, room := range cfg.Rooms {
		roomLabel := room.ID
		if name, ok := room.LocalizedName["pl"]; ok && name != "" {
			roomLabel = NormalizeName(name)
		} else if name, ok := room.LocalizedName["en"]; ok && name != "" {
			roomLabel = NormalizeName(name)
		}

		for _, devConf := range room.Entities {
			pc.deviceRoomMap[devConf.ID] = roomLabel
		}
	}

	return pc
}

// Describe sends the super-set of all possible descriptors of metrics
func (pc *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	// Since metrics are dynamic based on devices, we can't easily describe them all upfront 
	// without iterating devices again, but Describe is mostly for checking consistency.
	// We can leave this unchecked or implement if strictly needed, 
	// but unchecked collectors are common for dynamic metrics.
}

// Collect is called by the Prometheus registry when collecting metrics.
func (pc *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	devices := pc.vdevManager.Devices()

	for _, dev := range devices {
		if dev == nil {
			continue
		}

		val := 0.0
		isValid := false

		// Determine numeric value
		switch v := dev.State.(type) {
		case bool:
			if v {
				val = 1.0
			} else {
				val = 0.0
			}
			isValid = true
		case float64:
			val = v
			isValid = true
		case int:
			val = float64(v)
			isValid = true
		case int64:
			val = float64(v)
			isValid = true
		case string:
			lower := strings.ToLower(v)
			if lower == "on" {
				val = 1.0
				isValid = true
			} else if lower == "off" {
				val = 0.0
				isValid = true
			} else {
				// Try parsing float
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					val = parsed
					isValid = true
				}
			}
		}

		if !isValid {
			// Skip devices with non-numeric unknown state
			continue
		}


		roomID := pc.deviceRoomMap[dev.ID]
		// Metric name based on type
		metricName := "at2_" + string(dev.Type)

		help := "Virtual device metric for " + string(dev.Type)
		switch dev.Type {
		case VdevTypeRelay:
			help = "Relay state (0=off, 1=on)"
		case VdevTypeContact:
			help = "Contact sensor state (0=open, 1=closed)"
		case VdevTypeTemperature:
			help = "Temperature in Celsius"
		case VdevTypeHumidity:
			help = "Humidity in %"
		case VdevTypeCo:
			help = "Carbon Monoxide level in ppm"
		case VdevTypeGas:
			help = "Gas level in LEL"
		case VdevTypePowerUsage:
			help = "Power usage in Watts"
		}

		desc := prometheus.NewDesc(
			metricName,
			help,
			[]string{"id", "room"},
			nil,
		)

		ch <- prometheus.MustNewConstMetric(
			desc,
			prometheus.GaugeValue,
			val,
			dev.ID,
			roomID,
		)
	}
}
