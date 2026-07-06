package main

import (
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// requirePrinterStream rejects the request before the WebSocket upgrade when
// no stream is registered for the printer.
func requirePrinterStream(c *fiber.Ctx) error {
	printerID := c.Params("+")
	if printerStreamRegistry == nil {
		return fiber.ErrNotFound
	}
	// "+" is a greedy param so printer IDs containing slashes are captured whole.
	if _, ok := printerStreamRegistry.Manager(printerID); !ok {
		return fiber.ErrNotFound
	}
	// Preserve the resolved printer id for the upgraded WebSocket context.
	c.Locals("printer_id", printerID)
	return c.Next()
}

// handlePrinterStreamWs streams fMP4 video for one printer to one client.
// Frames come pre-encoded from the StreamManager (see stream_manager.go for
// the wire protocol); this handler just pumps them out.
func handlePrinterStreamWs(c *websocket.Conn) {
	printerID, _ := c.Locals("printer_id").(string)
	if printerID == "" {
		// Fallback for compatibility if middleware did not set Locals.
		printerID = c.Params("+")
	}

	defer c.Close()

	mgr, ok := printerStreamRegistry.Manager(printerID)
	if !ok {
		return
	}

	sub := mgr.Subscribe()
	defer mgr.Unsubscribe(sub)

	// Read loop: we expect no client messages, but reading is required to
	// notice the connection closing.
	clientGone := make(chan struct{})
	go func() {
		defer close(clientGone)
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case frame, ok := <-sub.C:
			if !ok {
				return
			}
			c.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.WriteMessage(websocket.BinaryMessage, frame); err != nil {
				return
			}
		case <-clientGone:
			return
		}
	}
}
