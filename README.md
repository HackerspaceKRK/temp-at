# temp-at

Web app used by [Hackerspace Kraków](https://hackerspace-krk.pl/)

## Features
- Displays snapshots

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

