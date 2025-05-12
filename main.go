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
)

func main() {
	FRIGATE_URL = os.Getenv("FRIGATE_URL")
	if FRIGATE_URL == "" {
		log.Fatal("FRIGATE_URL environment variable is required")
	}
	PORT = os.Getenv("LISTEN_PORT")
	if PORT == "" {
		PORT = ":8080"
	}
	go refreshImagesPeriodically()

	http.HandleFunc("/", serveWebpage)
	http.HandleFunc("/image/", serveImage)
	http.HandleFunc("/robots.txt", serveRobots)

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
	resp, err := http.Get(FRIGATE_URL + "/api/config")
	if err != nil {
		log.Println("Error fetching config:", err)
		return
	}
	defer resp.Body.Close()

	var config struct {
		Cameras map[string]map[string]interface{} `json:"cameras"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		log.Println("Error decoding config JSON:", err)
		return
	}

	for name, camConfig := range config.Cameras {
		if disabled, ok := camConfig["disabled"].(bool); ok && disabled {
			continue
		}
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
