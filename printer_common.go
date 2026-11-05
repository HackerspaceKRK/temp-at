package main

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// PrinterType tells the frontend which vendor-specific fields a printer state
// carries (and therefore which panels to render).
type PrinterType string

const (
	PrinterTypeBambu    PrinterType = "bambu"
	PrinterTypeAnycubic PrinterType = "anycubic"
)

// PrinterBaseState holds the vendor-neutral part of a printer vdev state. Vendor
// states (BambuPrinterState, AnycubicPrinterState) embed it, so its fields are
// flattened into the same JSON object as the vendor-specific ones.
type PrinterBaseState struct {
	Type PrinterType `json:"type"`
	// State is one of: idle, printing, paused, finished, failed, offline.
	State         string `json:"state"`
	Progress      int    `json:"progress"`       // percent, 0-100
	RemainingTime int    `json:"remaining_time"` // minutes
	Filename      string `json:"filename"`
	// PrintError is a vendor error code ("" when there is no error).
	PrintError string `json:"print_error"`
	// PrintErrorText is a human-readable description of the error / failure
	// reason; "" when there is none or it is unknown.
	PrintErrorText string `json:"print_error_text"`
	// TaskID identifies the current/last print; push subscriptions are bound to it.
	TaskID        string `json:"task_id"`
	LayerNum      int    `json:"layer_num"`
	TotalLayerNum int    `json:"total_layer_num"`
	// StartedAt is when the current/last print started (unix millis, 0 if unknown).
	StartedAt int64 `json:"started_at"`
	// FinishedAt is when the print finished or failed (unix millis, 0 while running).
	FinishedAt int64 `json:"finished_at"`
	Online     bool  `json:"online"`
	// HasThumbnail is true once a preview image for the current print has been
	// fetched and cached; the frontend then loads /api/v1/printer-thumbnail/<id>.
	HasThumbnail bool `json:"has_thumbnail"`

	// SnapshotImages/LowResPreview mirror the Frigate camera snapshot pattern:
	// auth-gated URL variants plus a tiny inline preview safe for the public WS.
	SnapshotImages []SnapshotImage `json:"snapshot_images,omitempty"`
	LowResPreview  string          `json:"low_res_preview,omitempty"`
	// HasStream is true when a live video stream endpoint exists for this printer.
	HasStream bool `json:"has_stream"`
}

func (s PrinterBaseState) Active() bool {
	return s.State == "printing" || s.State == "paused"
}

// sendPrintFinishedNotification fires a web push when a print transitions from
// active to finished/failed. failDetail (e.g. " — Nozzle clog") is appended to
// the failure message.
func sendPrintFinishedNotification(push *PushService, footerName, printerID string, prev, cur PrinterBaseState, failDetail string) {
	if push == nil || !prev.Active() {
		return
	}
	name := cur.Filename
	if name == "" {
		name = "print"
	}
	started := ""
	if cur.StartedAt > 0 {
		started = " Started " + formatPrintTime(cur.StartedAt) + "."
	}
	title := func(base string) string {
		if footerName != "" {
			return footerName + " · " + base
		}
		return base
	}
	now := formatPrintTime(time.Now().UnixMilli())
	switch cur.State {
	case "finished":
		push.SendPrintNotification(printerID, prev.TaskID, title("Print finished ✅"),
			fmt.Sprintf("%q finished at %s.%s", name, now, started))
	case "failed":
		push.SendPrintNotification(printerID, prev.TaskID, title("Print failed ❌"),
			fmt.Sprintf("%q failed%s at %s.%s", name, failDetail, now, started))
	}
}

// formatPrintTime renders a unix-millis timestamp as a short local datetime.
func formatPrintTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04")
}

// currentPrintTaskID returns the task id of the print running on any printer
// service (empty if unknown). Used when recording a push subscription.
func currentPrintTaskID(printerID string) string {
	if bambuService != nil {
		if id := bambuService.CurrentTaskID(printerID); id != "" {
			return id
		}
	}
	if anycubicService != nil {
		return anycubicService.CurrentTaskID(printerID)
	}
	return ""
}

// handlePrinterThumbnail serves the cached preview PNG of a printer's current
// print, whichever vendor service owns the printer.
func handlePrinterThumbnail(c *fiber.Ctx) error {
	// "+" is a greedy param so printer IDs containing slashes (e.g.
	// "bambu/cnc/printer") are captured whole.
	id := c.Params("+")
	var png []byte
	if bambuService != nil {
		png, _ = bambuService.CurrentThumbnail(id)
	}
	if len(png) == 0 && anycubicService != nil {
		png, _ = anycubicService.CurrentThumbnail(id)
	}
	if len(png) == 0 {
		return fiber.ErrNotFound
	}
	c.Set("Content-Type", "image/png")
	c.Set("Cache-Control", "no-cache")
	return c.Status(fiber.StatusOK).Send(png)
}
