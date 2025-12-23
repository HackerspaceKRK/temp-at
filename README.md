# temp-at

A Go + React application for home automation and room monitoring.

## Features
- **Room Monitoring**: View room states, camera feeds, and history.
- **Device Control**: Control MQTT devices via the web interface.
- **Authentication**: OIDC integration for secure access.
- **Localization**: Support for multiple languages.

## Getting Started

### Prerequisites
- Go 1.21+
- Node.js & npm (for building the frontend)

### Configuration
Copy `at2.example.yaml` to `at2.yaml` and adjust the settings:
```bash
cp at2.example.yaml at2.yaml
```

### Running Locally (Dev Mode)
1. Start the backend with the frontend in dev mode:
```bash
go run . -dev-frontend
```
2. The browser will open or navigate to `http://localhost:8080`. Frontend changes will hot-reload.

### Building for Production
1. Build the application (this includes building the frontend and embedding it):
```bash
go generate ./...
go build -o temp-at
```
2. Run the binary:
```bash
./temp-at
```

### Docker
Build the container:
```bash
docker build -t temp-at .
```

Run the container:
```bash
docker run -v $(pwd)/at2.yaml:/at2.yaml -v $(pwd)/data:/data -p 8080:8080 temp-at
```

## Secrets
Sensitive configuration (passwords, secrets) can be provided directly in `at2.yaml` or loaded from files using the `*_file` config options. This is useful for Docker Swarm or Kubernetes secrets.

- `mqtt.password` / `mqtt.password_file`
- `oidc.client_secret` / `oidc.client_secret_file`
- `web.jwt_secret` / `web.jwt_secret_file`
