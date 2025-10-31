package main

import (
	_ "embed" // for embedding template
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http" // for http.TimeFormat
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/gofiber/contrib/websocket"
)

var FRIGATE_URL string
var PORT string

type CameraImage struct {
	Data      []byte
	Timestamp time.Time
}

var (
	cameraImages = sync.Map{} // map[string]CameraImage
	mu           = sync.Mutex{}
	mqttAdapter  *MQTTAdapter
)

//go:embed template.html
var templateHTML string

var tpl = template.Must(template.New("webpage").Parse(templateHTML))

func main() {
	cfg := MustLoadConfig()
	FRIGATE_URL = cfg.Frigate.Url
	if override := os.Getenv("FRIGATE_URL"); override != "" {
		FRIGATE_URL = override
	}
	if FRIGATE_URL == "" {
		log.Fatal("frigate.url must be set in at2.yaml or FRIGATE_URL env var")
	}
	PORT = os.Getenv("LISTEN_PORT")
	if PORT == "" {
		PORT = ":8080"
	}

	var err error
	mqttAdapter, err = NewMQTTAdapter(cfg, log.Default())
	if err != nil {
		log.Fatalf("failed to initialize MQTT adapter: %v", err)
	}

	go refreshImagesPeriodically()

	app := fiber.New()

	// Routes
	app.Get("/", handleWebpage)
	app.Get("/image/:name", handleImage)
	app.Get("/robots.txt", handleRobots)
	app.Get("/api/v1/all-devices", handleDevices)
	app.Get("/api/v1/live-ws", websocket.New(handleLiveWs))
	app.Get("/api/v1/room-states", handleGetRoomStates)

	log.Printf("Serving on http://localhost%s", PORT)
	if err := app.Listen(PORT); err != nil {
		log.Fatalf("Fiber server failed: %v", err)
	}
}

func refreshImagesPeriodically() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		// updateCameraImages()
		<-ticker.C
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

func handleWebpage(c *fiber.Ctx) error {
	names := make([]string, 0)
	cameraImages.Range(func(key, value interface{}) bool {
		names = append(names, key.(string))
		return true
	})

	c.Set("Content-Type", "text/html")
	c.Set("Cache-Control", "no-cache")
	c.Set("Last-Modified", time.Now().Format(http.TimeFormat))

	var buf strings.Builder
	if err := tpl.Execute(&buf, map[string]interface{}{
		"Cameras": names,
	}); err != nil {
		log.Println("Error rendering template:", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error rendering template")
	}
	return c.Status(fiber.StatusOK).SendString(buf.String())
}

func handleDevices(c *fiber.Ctx) error {
	if mqttAdapter == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("MQTT adapter not initialized")
	}
	devices := mqttAdapter.VirtualDevices()

	c.Set("Cache-Control", "no-cache")
	return c.Status(fiber.StatusOK).JSON(devices)
}
