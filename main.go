package main

import (
	_ "embed" // for embedding template
	"flag"
	"fmt"
	"io"
	"log"
	"net/http" // for http.TimeFormat
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
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
	_ = NewVirtualDeviceHistoryRepository(db, vdevManager)

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

	app := fiber.New()

	// Routes
	app.Get("/image/:name", handleImage)
	app.Get("/robots.txt", handleRobots)
	app.Get("/api/v1/all-devices", handleDevices)
	app.Get("/api/v1/live-ws", websocket.New(handleLiveWs))
	app.Get("/api/v1/room-states", handleGetRoomStates)
	app.Get("/api/v1/camera-snapshot/:filename", frigateSnapshotMapper.HandleSnapshot)
	app.Get("/api/v1/auth/login", handleLoginRequest)
	app.Get("/api/v1/auth/callback", handleAuthCallback)
	app.Get("/api/v1/auth/me", handleMe)
	app.Post("/api/v1/auth/logout", handleLogout)

	if *devFrontend {
		log.Println("Starting frontend in dev mode...")
		cmd := exec.Command("pnpm", "dev", "--host", "0.0.0.0")
		cmd.Dir = "./at2-web"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Fatalf("Failed to start frontend: %v", err)
		}
		defer func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}()

		app.All("*", func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/api") {
				return c.Next()
			}
			proxyUrl := "http://localhost:5173" + c.Path()
			return proxy.Do(c, proxyUrl)
		})
	}

	log.Printf("Starting Fiber server on %s", cfg.Web.ListenAddress)
	if err := app.Listen(cfg.Web.ListenAddress); err != nil {
		log.Fatalf("Fiber server failed: %v", err)
	}
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
