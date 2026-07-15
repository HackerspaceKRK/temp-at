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
| `bambu_service.go` | Bambu Labs printer monitoring: one TLS MQTT client per printer, merges the device report into a `BambuPrinterState` vdev (never persisted; includes AMS trays, fans, wifi, HMS errors), fires push notifications on print finish/failure |
| `printer_stream.go` / `stream_manager.go` | Vendor-neutral printer camera streaming: `PrinterVideoSource` interface (implement per vendor) + `StreamManager` that keeps **at most one** camera connection per printer, muxes H.264 into fMP4 and fans it out to WebSocket viewers (`/api/v1/printer-stream/+`, auth-only, MSE playback) |
| `bambu_stream.go` | Bambu `PrinterVideoSource`: RTSPS chamber camera via gortsplib (`rtsps://bblp:<access_code>@host:322/streaming/live/1`, same creds as MQTT) |
| `printer_snapshots.go` / `snapshot_store.go` | Once a minute, grab a keyframe from each online printer via the StreamManager (reuses a running live stream), decode to JPEG by shelling out to `ffmpeg` (skipped gracefully when not installed), resize via the shared `SnapshotStore` (also used by the Frigate mapper) and publish into the printer vdev state |
| `push_service.go` / `push_handlers.go` | Web Push (VAPID) — keys persisted in DB (`AppSettingModel`), per-print subscriptions (`PushSubscriptionModel`), `/api/v1/push/*` endpoints |
| `exit_board_service.go` | Publishes a per-room status code (0/1/2) to `<prefix>/<room_id>` over the main MQTT connection for an exit-status light panel; reacts to vdev state changes. Lights = `representation: light` (relay `ON`/`OFF`), windows = any `contact`-type vdev in the room (regardless of representation) |

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

### Adding a New Printer Type (e.g. OctoPrint)

Printer live streams and snapshots are vendor-neutral. Implement
`PrinterVideoSource` (`printer_stream.go`) for the camera and register it in a
`PrinterStreamRegistry`; publish a state shaped like `BambuPrinterState` into a
vdev of type `printer`, and implement `PrinterSnapshotSink`
(`printer_snapshots.go`) for periodic snapshots. The WS endpoint, StreamManager,
snapshotter and the whole frontend (`/printers`, popover, AMS panel) then work
unchanged.

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
- `bambu_printers` — optional list of Bambu Labs printers monitored over their local TLS MQTT interface; reference a printer's `id` from a room entity with `representation: printer` to show a status popover + web push notifications. Printers also appear on the `/printers` page with AMS filament info, minute camera snapshots and a login-gated live stream (`rtsp_port`, default 322)
- `exit_board` — optional; set `mqtt_prefix` to publish a per-room status code (0/1/2) to `<mqtt_prefix>/<room_id>` for an exit light panel. Lights = entities with `representation: light`, windows = any contact sensor in the room
- `nav_links` — optional list of custom navbar links (`name`, optional `localized_name`, `url`); each opens in a new tab with a lucide external-link icon (main navbar only, not kiosk)

### CI/CD

`.github/workflows/build-docker.yml` builds and pushes a Docker image to GHCR on every push to `master`.
