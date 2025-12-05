package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/image/draw"
)

type FrigateSnapshotMapperData struct {
	CameraName string `json:"camera_name"`
}

type SnapshotImage struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	MediaType string `json:"media_type"`
}

type FrigateSnapshotState struct {
	Images []SnapshotImage `json:"images"`
}

// FrigateSnapshotMapper communicates with frigate over it's HTTP API to discover cameras
// and periodically fetch snapshot images.
// It then exposes them as VirtualDevices of type "camera_snapshot".
type FrigateSnapshotMapper struct {
	vdevMgr *VdevManager
	cfg     *Config

	cameraNames []string
	imagesCache map[string][]byte

	mu sync.RWMutex
}

func NewFrigateSnapshotMapper(vdevMgr *VdevManager, cfg *Config) *FrigateSnapshotMapper {
	return &FrigateSnapshotMapper{
		vdevMgr:     vdevMgr,
		cfg:         cfg,
		cameraNames: []string{},
		imagesCache: map[string][]byte{},
	}
}

func (s *FrigateSnapshotMapper) Start() error {

	err := s.fetchCameraNames()
	if err != nil {
		return fmt.Errorf("failed to fetch camera names from frigate: %w", err)
	}

	vdevs := []*VirtualDevice{}
	for _, name := range s.cameraNames {
		vdev := &VirtualDevice{
			ID:    fmt.Sprintf("snapshot/%s", name),
			State: nil,
			Type:  "camera_snapshot",
			MapperData: FrigateSnapshotMapperData{
				CameraName: name,
			},
		}

		vdevs = append(vdevs, vdev)
	}
	s.vdevMgr.AddDevices(vdevs)

	go s.fetchLoop()

	return nil

}

func (s *FrigateSnapshotMapper) fetchLoop() {
	// Fetch snapshots every minute
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		updates := []*VirtualDeviceUpdate{}
		for _, name := range s.cameraNames {
			images, error := s.fetchCameraSnapshot(name)
			if error != nil {
				log.Printf("[frigate snapshot mapper] failed to fetch snapshot for camera %s: %v", name, error)
				continue
			}

			updates = append(updates, &VirtualDeviceUpdate{
				Name: fmt.Sprintf("snapshot/%s", name),
				State: FrigateSnapshotState{
					Images: images,
				},
			})
		}
		s.vdevMgr.ApplyUpdates(updates)
		<-ticker.C
	}
}

func (s *FrigateSnapshotMapper) fetchCameraSnapshot(cameraName string) ([]SnapshotImage, error) {
	// Refactored:
	// 1. Fetch snapshot ONCE as JPEG from Frigate.
	// 2. Decode locally using stdlib image/jpeg.
	// 3. Resize to widths 300, 600, 900 (maintain aspect ratio) + original.
	// 4. Encode each variant as JPEG
	// 5. Store in s.imagesCache and return metadata with cache-busting URL.
	base := strings.TrimRight(s.cfg.Frigate.Url, "/")
	if base == "" {
		return nil, fmt.Errorf("frigate url empty")
	}
	s.mu.Lock()
	if s.imagesCache == nil {
		s.imagesCache = make(map[string][]byte)
	}
	s.mu.Unlock()

	ts := time.Now().Unix()
	origURL := fmt.Sprintf("%s/api/%s/latest.jpg?cache=%d&height=1080", base, cameraName, ts)
	resp, err := http.Get(origURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch original snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("frigate snapshot status %d", resp.StatusCode)
	}
	origBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot body failed: %w", err)
	}

	srcImg, err := jpeg.Decode(bytes.NewReader(origBytes))
	if err != nil {
		return nil, fmt.Errorf("jpeg decode failed: %w", err)
	}

	origBounds := srcImg.Bounds()
	origW := origBounds.Dx()
	origH := origBounds.Dy()

	log.Printf("[frigate snapshot mapper] fetched snapshot for camera %s: %dx%d", cameraName, origW, origH)

	images := []SnapshotImage{}

	storeVariant := func(width, height int, ext string, data []byte) {
		widthPart := "orig"
		if width > 0 {
			widthPart = fmt.Sprintf("%d", width)
		}
		filename := fmt.Sprintf("%s_%s.%s", cameraName, widthPart, ext)

		s.mu.Lock()
		s.imagesCache[filename] = data
		s.mu.Unlock()
		images = append(images, SnapshotImage{
			URL:       fmt.Sprintf("/api/v1/camera-snapshot/%s?cache=%d", filename, ts),
			Width:     width,
			Height:    height,
			MediaType: "image/" + ext,
		})
	}

	// Store original as-is.
	storeVariant(origW, origH, "jpg", origBytes)

	targetWidths := []int{300, 600, 900}
	for _, w := range targetWidths {
		if w <= 0 || w >= origW {
			continue
		}
		h := int(float64(origH) * (float64(w) / float64(origW)))
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), srcImg, origBounds, draw.Over, nil)

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
			continue
		}
		storeVariant(w, h, "jpg", buf.Bytes())
	}

	return images, nil
}

// GetCachedSnapshot returns the image bytes and media type for a given snapshot filename.
// If the requested variant is not present in the cache, it triggers a refresh for the camera
// and retries the lookup once.
func (s *FrigateSnapshotMapper) GetCachedSnapshot(filename string) ([]byte, string, error) {
	if filename == "" {
		return nil, "", fmt.Errorf("empty filename")
	}

	cacheKey := filename
	s.mu.RLock()
	data, ok := s.imagesCache[cacheKey]
	s.mu.RUnlock()
	if ok {
		return data, "image/jpeg", nil
	}
	return nil, "", fmt.Errorf("snapshot not found in cache")
}

// HandleSnapshot is an HTTP handler for Fiber that serves a cached snapshot variant.
func (s *FrigateSnapshotMapper) HandleSnapshot(c *fiber.Ctx) error {
	filename := c.Params("filename")
	data, mediaType, err := s.GetCachedSnapshot(filename)
	if err != nil || len(data) == 0 {
		return fiber.ErrNotFound
	}
	c.Set("Content-Type", mediaType)
	c.Set("Cache-Control", "no-cache")
	c.Set("Content-Length", fmt.Sprintf("%d", len(data)))
	return c.Status(fiber.StatusOK).Send(data)
}

// Removed nearest-neighbor helper; using golang.org/x/image/draw CatmullRom for resizing.

// FrigateConfigResponse is an incomplete schema for the /api/config response from Frigate.
type FrigateConfigResponse struct {
	Cameras map[string]any `json:"cameras"`
}

func (s *FrigateSnapshotMapper) fetchCameraNames() error {
	base := strings.TrimRight(s.cfg.Frigate.Url, "/")
	if base == "" {
		return fmt.Errorf("frigate url empty")
	}
	resp, err := http.Get(base + "/api/config")
	if err != nil {
		return fmt.Errorf("frigate /api/config request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("frigate /api/config unexpected status: %d", resp.StatusCode)
	}

	var cfgResp FrigateConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfgResp); err != nil {
		return fmt.Errorf("failed to decode frigate config response: %w", err)
	}

	names := make([]string, 0, len(cfgResp.Cameras))
	for name := range cfgResp.Cameras {
		names = append(names, name)
	}
	sort.Strings(names)
	s.cameraNames = names
	return nil
}
