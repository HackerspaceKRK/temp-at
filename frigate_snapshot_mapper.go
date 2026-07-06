package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

type FrigateSnapshotMapperData struct {
	CameraName string `json:"camera_name"`
}

const LowResThumbnailSize = 64

type SnapshotImage struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	MediaType string `json:"media_type"`
}

type FrigateSnapshotState struct {
	Images        []SnapshotImage `json:"images"`
	LowResPreview string          `json:"low_res_preview"`
}

// FrigateSnapshotMapper communicates with frigate over it's HTTP API to discover cameras
// and periodically fetch snapshot images.
// It then exposes them as VirtualDevices of type "camera_snapshot".
type FrigateSnapshotMapper struct {
	vdevMgr *VdevManager
	cfg     *Config
	store   *SnapshotStore

	cameraNames []string
}

func NewFrigateSnapshotMapper(vdevMgr *VdevManager, cfg *Config, store *SnapshotStore) *FrigateSnapshotMapper {
	return &FrigateSnapshotMapper{
		vdevMgr:     vdevMgr,
		cfg:         cfg,
		store:       store,
		cameraNames: []string{},
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
			Type:  VdevTypeCameraSnapshot,
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
			images, lowResPreview, error := s.fetchCameraSnapshot(name)
			if error != nil {
				log.Printf("[frigate snapshot mapper] failed to fetch snapshot for camera %s: %v", name, error)
				continue
			}

			updates = append(updates, &VirtualDeviceUpdate{
				Name: fmt.Sprintf("snapshot/%s", name),
				State: FrigateSnapshotState{
					Images:        images,
					LowResPreview: lowResPreview,
				},
			})
		}
		s.vdevMgr.ApplyUpdates(updates)
		<-ticker.C
	}
}

// fetchCameraSnapshot fetches the latest JPEG from Frigate once and delegates
// resizing/caching of the width variants to the shared SnapshotStore.
func (s *FrigateSnapshotMapper) fetchCameraSnapshot(cameraName string) ([]SnapshotImage, string, error) {
	base := strings.TrimRight(s.cfg.Frigate.Url, "/")
	if base == "" {
		return nil, "", fmt.Errorf("frigate url empty")
	}

	ts := time.Now().Unix()
	origURL := fmt.Sprintf("%s/api/%s/latest.jpg?cache=%d&height=1080", base, cameraName, ts)
	resp, err := http.Get(origURL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch original snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("frigate snapshot status %d", resp.StatusCode)
	}
	origBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading snapshot body failed: %w", err)
	}

	return s.store.BuildVariants(origBytes, cameraName, ts)
}

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
