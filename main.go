package main

import (
	_ "embed" // for embedding template
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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

	mqttAdapter = mqttAdapter

	go refreshImagesPeriodically()

	http.HandleFunc("/", serveWebpage)
	http.HandleFunc("/image/", serveImage)
	http.HandleFunc("/robots.txt", serveRobots)
	http.HandleFunc("/devices", serveDevices)

	log.Printf("Serving on http://localhost%s", PORT)
	log.Fatal(http.ListenAndServe(PORT, nil))
}

func refreshImagesPeriodically() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		updateCameraImages()
		<-ticker.C
	}
}

func updateCameraImages() {
	cfg := GetConfig()
	if cfg == nil {
		log.Println("Config not loaded yet")
		return
	}

	// Collect unique camera names from config rooms
	unique := map[string]struct{}{}
	for _, room := range cfg.Rooms {
		for _, cam := range room.Cameras {
			if cam == "" {
				continue
			}
			unique[cam] = struct{}{}
		}
	}

	if len(unique) == 0 {
		log.Println("No cameras defined in config rooms")
		return
	}

	for name := range unique {
		go fetchAndCacheImage(name)
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

func serveImage(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/image/")
	if val, ok := cameraImages.Load(name); ok {
		img := val.(CameraImage)
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Last-Modified", img.Timestamp.Format(http.TimeFormat))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(img.Data)))
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(img.Data); err != nil {
			log.Printf("Error writing image for camera %s: %v", name, err)
		}
	} else {
		http.NotFound(w, r)
	}
}

var robotsTxt = []byte(`User-agent: *
Disallow: /`)

func serveRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(robotsTxt)
}

//go:embed template.html
var templateHTML string

var tpl = template.Must(template.New("webpage").Parse(templateHTML))

func serveWebpage(w http.ResponseWriter, r *http.Request) {
	names := make([]string, 0)
	cameraImages.Range(func(key, value interface{}) bool {
		names = append(names, key.(string))
		return true
	})

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	if err := tpl.Execute(w, map[string]interface{}{
		"Cameras": names,
	}); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		log.Println("Error rendering template:", err)
		return
	}

}

func serveDevices(w http.ResponseWriter, r *http.Request) {
	if mqttAdapter == nil {
		http.Error(w, "MQTT adapter not initialized", http.StatusServiceUnavailable)
		return
	}
	devices := mqttAdapter.VirtualDevices()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(devices); err != nil {
		http.Error(w, "Failed to encode devices", http.StatusInternalServerError)
		log.Printf("Error encoding devices: %v", err)
		return
	}
}
