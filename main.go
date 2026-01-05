package main

import (
	_ "embed" // for embedding template
	"flag"
	"fmt"
	"io"
	"log"
	"net/http" // for http.TimeFormat
	"sync"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var FRIGATE_URL string
var PORT string

type CameraImage struct {
	Data      []byte
	Timestamp time.Time
}

var (
	cameraImages          = sync.Map{} // map[string]CameraImage
	vdevManager           *VdevManager
	mqttAdapter           *MQTTAdapter
	frigateSnapshotMapper *FrigateSnapshotMapper
	vdevHistoryRepo       *VirtualDeviceHistoryRepository
)

func main() {
	devFrontend := flag.Bool("dev-frontend", false, "Start frontend in dev mode")
	flag.Parse()

	cfg := MustLoadConfig()

	err := initAuth()
	if err != nil {
		log.Fatalf("failed to initialize authentication: %v", err)
	}

	vdevManager = NewVdevManager()

	// Initialize database
	db, err := gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := AutoMigrateModels(db); err != nil {
		log.Fatalf("failed to run database migrations: %v", err)
	}
	log.Printf("Database initialized at %s", cfg.Database.Path)

	// Create history repository (registers itself as listener)
	vdevHistoryRepo = NewVirtualDeviceHistoryRepository(db, vdevManager)

	mqttAdapter, err = NewMQTTAdapter(cfg, vdevManager)
	if err != nil {
		log.Fatalf("failed to initialize MQTT adapter: %v", err)
	}

	frigateSnapshotMapper = NewFrigateSnapshotMapper(vdevManager, cfg)
	err = frigateSnapshotMapper.Start()
	if err != nil {
		log.Fatalf("failed to start Frigate snapshot mapper: %v", err)
	}

	vdevManager.OnVirtualDeviceUpdated = append(
		vdevManager.OnVirtualDeviceUpdated,
		handleVirtualDeviceStateUpdate,
	)

	// Wire up the state provider for persistence restoration
	vdevManager.SetStateProvider(vdevHistoryRepo)

	app := fiber.New()

	// Prometheus
	promRegistry := prometheus.NewRegistry()
	promCollector := NewPrometheusCollector(vdevManager, cfg)
	promRegistry.MustRegister(promCollector)

	// Routes
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})))
	app.Get("/image/:name", AuthMiddleware, handleImage)
	app.Get("/robots.txt", handleRobots)
	app.Get("/api/v1/all-devices", handleDevices)
	app.Get("/api/v1/live-ws", websocket.New(handleLiveWs))
	app.Get("/api/v1/room-states", handleGetRoomStates)
	app.Get("/api/v1/camera-snapshot/:filename", AuthMiddleware, frigateSnapshotMapper.HandleSnapshot)
	app.Get("/api/v1/auth/login", handleLoginRequest)
	app.Get("/api/v1/auth/callback", handleAuthCallback)
	app.Get("/api/v1/auth/me", handleMe)
	app.Post("/api/v1/auth/logout", handleLogout)
	app.Post("/api/v1/control-relay", AuthMiddleware, handleControlRelay)
	app.Get("/api/v1/spaceapi", handleSpaceAPI)
	app.Get("/api/v1/branding", handleBranding)
	app.Get("/health", handleHealth)
	app.Get("/api/v1/device-history", handleDeviceHistory)
	app.Get("/api/v1/stats/usage-heatmap", handleUsageHeatmap)

	SetupFrontend(app, *devFrontend)

	log.Printf("Starting Fiber server on %s", cfg.Web.ListenAddress)
	if err := app.Listen(cfg.Web.ListenAddress); err != nil {
		log.Fatalf("Fiber server failed: %v", err)
	}
}

func handleDeviceHistory(c *fiber.Ctx) error {
	deviceName := c.Query("device")
	if deviceName == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Missing device query parameter")
	}

	// 24 hours in milliseconds
	duration := int64(24 * 60 * 60 * 1000)
	history, err := vdevHistoryRepo.GetDeviceHistory(deviceName, duration)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	return c.JSON(history)
}

type ControlRelayRequest struct {
	ID    string `json:"id"`
	State string `json:"state"` // "ON" or "OFF"
}

func handleControlRelay(c *fiber.Ctx) error {
	var req ControlRelayRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if mqttAdapter == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("MQTT adapter not initialized")
	}

	log.Printf("User %s requested to turn %s relay %s", c.Locals("username"), req.State, req.ID)

	if err := mqttAdapter.ControlDevice(req.ID, req.State); err != nil {
		// Differentiate between user error (bad ID/State) and system error?
		// For now, simpler to just return 400 or 500 based on error content,
		// but standardizing on 500 for control failures is acceptable for this scope
		// unless we strictly parse validation errors.
		// Given validation happens in ControlDevice, let's treat it as potentially bad request if verification fails.
		// Use 400 if validation error, 500 if MQTT error.
		// For simplicity/speed, returning 400 is safer for "invalid state" etc.
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

func fetchAndCacheImage(name string) {
	url := fmt.Sprintf("%s/api/%s/latest.webp?height=900&cache=%d", FRIGATE_URL, name, time.Now().Unix())
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Error fetching image for camera %s: %v", name, err)
		return
	}
	defer resp.Body.Close()

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading image for camera %s: %v", name, err)
		return
	}

	cameraImages.Store(name, CameraImage{
		Data:      imgBytes,
		Timestamp: time.Now(),
	})
}

func handleImage(c *fiber.Ctx) error {
	name := c.Params("name")
	if val, ok := cameraImages.Load(name); ok {
		img := val.(CameraImage)
		c.Set("Content-Type", "image/webp")
		c.Set("Cache-Control", "no-cache")
		c.Set("Last-Modified", img.Timestamp.Format(http.TimeFormat))
		c.Set("Content-Length", fmt.Sprintf("%d", len(img.Data)))
		return c.Status(fiber.StatusOK).Send(img.Data)
	}
	return fiber.ErrNotFound
}

var robotsTxt = []byte(`User-agent: *
Disallow: /`)

func handleRobots(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	c.Set("Cache-Control", "no-cache")
	return c.Status(fiber.StatusOK).Send(robotsTxt)
}

func handleDevices(c *fiber.Ctx) error {
	if mqttAdapter == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("MQTT adapter not initialized")
	}
	devices := vdevManager.Devices()

	c.Set("Cache-Control", "no-cache")
	return c.Status(fiber.StatusOK).JSON(devices)
}

func handleBranding(c *fiber.Ctx) error {
	cfg := MustLoadConfig()
	return c.JSON(cfg.Branding)
}

func handleHealth(c *fiber.Ctx) error {
	return c.SendString("OK")
}
