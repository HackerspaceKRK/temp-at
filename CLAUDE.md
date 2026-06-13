# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**temp-at** is a full-stack home automation dashboard for hackerspaces. It bridges multiple IoT protocols (MQTT, Zigbee2MQTT, Frigate NVR, ESPHome) into a unified web UI with OIDC authentication, real-time WebSocket updates, and historical timeseries data.

- **Backend:** Go 1.24+ with Fiber framework, GORM ORM, SQLite
- **Frontend:** React 19 + TypeScript, Vite, Tailwind CSS (in `at2-web/`)

## Common Commands

### Backend
```bash
go run . -dev-frontend    # Run backend in dev mode (proxies frontend to Vite)
go build -o temp-at       # Build production binary
go generate ./...         # Rebuild embedded frontend assets
go test ./...             # Run all Go tests
go test -run TestName .   # Run a single test
```

### Frontend
```bash
cd at2-web
npm install
npm run dev               # Vite dev server on :5173
npm run build             # Production build to dist/
```

### Docker
```bash
docker build -t temp-at .
```

## Architecture

### Backend Data Flow

```
MQTT Broker
  → MQTTAdapter (subscribes, dispatches messages)
    → MQTTMapper implementations (zigbee2mqtt, frigate, esphome)
      → VdevManager (thread-safe in-memory device state)
        → state change callbacks → WebSocket broadcast (live_ws.go)
        → VirtualDeviceHistoryRepository (SQLite timeseries)
```

### Key Backend Files

| File | Purpose |
|------|---------|
| `main.go` | Fiber server setup, all HTTP route definitions |
| `config.go` / `config_loader.go` | YAML config structs + loading (supports `_file` secret variants) |
| `vdev_manager.go` | Thread-safe virtual device state holder with callback system |
| `mqtt_adapter.go` | MQTT broker connection; routes messages to mappers |
| `mqtt_mapper_zigbee2mqtt.go` | Zigbee2MQTT device discovery + state parsing |
| `mqtt_mapper_frigate.go` | Frigate NVR person detection events |
| `mqtt_esphome_mapper.go` | ESPHome sensors and relays |
| `live_ws.go` | WebSocket handler for frontend real-time updates |
| `auth.go` | OIDC login/logout, session management, back-channel logout |
| `models.go` | GORM models: sessions, virtual devices, device state history |
| `usage_stats.go` | Room occupancy statistics from device history |
| `spaceapi.go` | SpaceAPI JSON endpoint |
| `prometheus.go` | Prometheus metrics export |
| `dhcp_service.go` | Background DHCP lease scraper: polls the router, persists lease lifecycle, enriches with connection info |
| `dhcp_sources.go` | Vendor-neutral source interfaces (`DhcpLeaseSource`, `WiredPortSource`, `WifiClientSource`) + `kind`-keyed factories |
| `dhcp_source_mikrotik.go` | MikroTik RouterOS lease source + bridge-host (MAC→port) source |
| `dhcp_source_unifi.go` | UniFi controller WiFi client source (MAC→AP/SSID/RSSI) |
| `dhcp_handlers.go` | `/api/v1/dhcp/leases` handler with per-group CIDR filtering |
| `oui.go` / `manuf.gz` | Embedded Wireshark OUI database for MAC→vendor lookup |

### Adding a New DHCP Switch/AP Vendor

The DHCP scraper consumes three vendor-neutral interfaces defined in `dhcp_sources.go`.
To support a new device, implement the relevant interface and register it by `kind`
in the matching factory (`NewDhcpLeaseSource`, `NewWiredPortSource`, `NewWifiClientSource`):
- `DhcpLeaseSource` — the router's DHCP lease table
- `WiredPortSource` — MAC → switch name + port (wired devices)
- `WifiClientSource` — MAC → AP name + SSID + signal (wireless clients)

Config selects implementations via the `kind` field under `dhcp.router` /
`dhcp.wired_sources` / `dhcp.wifi_sources`. The scraper prefers WiFi info over
wired when a MAC appears in both (a wired hit for a wireless client is just the
AP's uplink port).

To refresh the embedded OUI database: download
`https://www.wireshark.org/download/automated/data/manuf` and run
`gzip -9 -c manuf > manuf.gz` in the repo root.

### Adding a New MQTT Mapper

Implement the `MQTTMapper` interface:
- `SubscriptionTopics() []string` — MQTT topics to subscribe to
- `DiscoverDevicesFromMessage(topic, payload) []VirtualDevice` — parse discovery messages
- `UpdateDevicesFromMessage(topic, payload) []VirtualDeviceUpdate` — parse state updates
- `Control(device, action) error` — send control commands back to broker

Register the mapper in `mqtt_adapter.go`.

### Frontend

- `src/app.tsx` — root layout, room grid
- `src/AppConfigContext.tsx` / `src/AuthContext.tsx` — global state via React Context
- `src/useWebsocket.tsx` — WebSocket hook that receives live device updates
- `src/schema.ts` — TypeScript types (mirrors Go API response shapes)
- `src/components/` — UI components (RoomCard, RelayControl, CameraSnapshot, etc.)

### Configuration

Copy `at2.example.yaml` → `at2.yaml`. Key sections:
- `mqtt` — broker address + credentials (supports `password_file`)
- `rooms` — room definitions with `entities` (devices) and `cameras`
- `oidc` — optional OIDC provider for authentication
- `spaceapi` — hackerspace metadata
- `branding` — logo/favicon/footer customization
- `dhcp` — optional DHCP lease tracking (router/switch/WiFi sources + per-group CIDR access control)

### CI/CD

`.github/workflows/build-docker.yml` builds and pushes a Docker image to GHCR on every push to `master`.
