package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"sync"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/image/draw"
)

// SnapshotStore is the shared in-memory cache of camera snapshot variants
// (Frigate cameras and printer cameras alike), served over
// /api/v1/camera-snapshot/:filename behind AuthMiddleware.
type SnapshotStore struct {
	mu     sync.RWMutex
	images map[string][]byte
}

func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{images: make(map[string][]byte)}
}

// BuildVariants decodes a source JPEG, resizes it to the standard width
// variants (300/600/900 + original) plus a tiny base64 preview, stores the
// variants under keyPrefix and returns their metadata with cache-busting URLs.
func (s *SnapshotStore) BuildVariants(srcJPEG []byte, keyPrefix string, ts int64) ([]SnapshotImage, string, error) {
	srcImg, err := jpeg.Decode(bytes.NewReader(srcJPEG))
	if err != nil {
		return nil, "", fmt.Errorf("jpeg decode failed: %w", err)
	}

	origBounds := srcImg.Bounds()
	origW := origBounds.Dx()
	origH := origBounds.Dy()

	images := []SnapshotImage{}

	storeVariant := func(width, height int, ext string, data []byte) {
		widthPart := "orig"
		if width > 0 {
			widthPart = fmt.Sprintf("%d", width)
		}
		filename := fmt.Sprintf("%s_%s.%s", keyPrefix, widthPart, ext)

		s.mu.Lock()
		s.images[filename] = data
		s.mu.Unlock()
		images = append(images, SnapshotImage{
			URL:       fmt.Sprintf("/api/v1/camera-snapshot/%s?cache=%d", filename, ts),
			Width:     width,
			Height:    height,
			MediaType: "image/" + ext,
		})
	}

	// Store original as-is.
	storeVariant(origW, origH, "jpg", srcJPEG)

	targetWidths := []int{300, 600, 900}
	for _, w := range targetWidths {
		if w <= 0 || w >= origW {
			continue
		}
		h := int(float64(origH) * (float64(w) / float64(origW)))
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), srcImg, origBounds, draw.Over, nil)

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
			continue
		}
		storeVariant(w, h, "jpg", buf.Bytes())
	}

	// Generate low-res preview (max LowResThumbnailSize px in any dimension).
	lowResW, lowResH := origW, origH
	if origW > origH {
		lowResW = LowResThumbnailSize
		lowResH = int(float64(origH) * float64(LowResThumbnailSize) / float64(origW))
	} else {
		lowResH = LowResThumbnailSize
		lowResW = int(float64(origW) * float64(LowResThumbnailSize) / float64(origH))
	}
	if lowResW == 0 {
		lowResW = 1
	}
	if lowResH == 0 {
		lowResH = 1
	}

	dstLow := image.NewRGBA(image.Rect(0, 0, lowResW, lowResH))
	draw.ApproxBiLinear.Scale(dstLow, dstLow.Bounds(), srcImg, origBounds, draw.Over, nil)

	var lowResBuf bytes.Buffer
	var lowResPreview string
	if err := jpeg.Encode(&lowResBuf, dstLow, &jpeg.Options{Quality: 70}); err == nil {
		lowResPreview = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(lowResBuf.Bytes())
	}

	return images, lowResPreview, nil
}

// Get returns the image bytes and media type for a given snapshot filename.
func (s *SnapshotStore) Get(filename string) ([]byte, string, error) {
	if filename == "" {
		return nil, "", fmt.Errorf("empty filename")
	}
	s.mu.RLock()
	data, ok := s.images[filename]
	s.mu.RUnlock()
	if ok {
		return data, "image/jpeg", nil
	}
	return nil, "", fmt.Errorf("snapshot not found in cache")
}

// HandleSnapshot is an HTTP handler for Fiber that serves a cached snapshot variant.
func (s *SnapshotStore) HandleSnapshot(c *fiber.Ctx) error {
	data, mediaType, err := s.Get(c.Params("filename"))
	if err != nil || len(data) == 0 {
		return fiber.ErrNotFound
	}
	c.Set("Content-Type", mediaType)
	c.Set("Cache-Control", "no-cache")
	c.Set("Content-Length", fmt.Sprintf("%d", len(data)))
	return c.Status(fiber.StatusOK).Send(data)
}
