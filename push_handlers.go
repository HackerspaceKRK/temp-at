package main

import (
	"github.com/gofiber/fiber/v2"
)

// handlePushVapidKey returns the server's VAPID public key for the frontend to
// build a PushManager subscription.
func handlePushVapidKey(c *fiber.Ctx) error {
	if pushService == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("push not available")
	}
	return c.JSON(fiber.Map{"key": pushService.PublicKey()})
}

type pushSubscribeRequest struct {
	PrinterID    string `json:"printer_id"`
	Subscription struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	} `json:"subscription"`
}

// handlePushSubscribe records a browser push subscription to be notified about
// the print currently running on the given printer.
func handlePushSubscribe(c *fiber.Ctx) error {
	if pushService == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("push not available")
	}
	var req pushSubscribeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	if req.PrinterID == "" || req.Subscription.Endpoint == "" ||
		req.Subscription.Keys.P256dh == "" || req.Subscription.Keys.Auth == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing printer_id or subscription fields")
	}

	taskID := ""
	if bambuService != nil {
		taskID = bambuService.CurrentTaskID(req.PrinterID)
	}

	if err := pushService.Subscribe(req.PrinterID, taskID,
		req.Subscription.Endpoint, req.Subscription.Keys.P256dh, req.Subscription.Keys.Auth); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.SendStatus(fiber.StatusOK)
}

type pushUnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// handlePushUnsubscribe removes a stored push subscription by endpoint.
func handlePushUnsubscribe(c *fiber.Ctx) error {
	if pushService == nil {
		return c.Status(fiber.StatusServiceUnavailable).SendString("push not available")
	}
	var req pushUnsubscribeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	if req.Endpoint == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing endpoint")
	}
	if err := pushService.Unsubscribe(req.Endpoint); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return c.SendStatus(fiber.StatusOK)
}
