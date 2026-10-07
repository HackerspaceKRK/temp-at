package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Emergency entry access states reported to the frontend. Checks are applied in
// this order; the first failing one is reported.
const (
	emergencyEntryDisabled          = "disabled"
	emergencyEntryOutsideNetwork    = "outside_network"
	emergencyEntryNotLoggedIn       = "not_logged_in"
	emergencyEntryForbidden         = "forbidden"
	emergencyEntryMembershipExpired = "membership_expired"
	emergencyEntryOK                = "ok"
)

const emergencyEntryRequestTimeout = 10 * time.Second

var (
	emergencyEntryNets      []*net.IPNet
	emergencyEntryTemplates map[string]*template.Template // door id -> URL template
)

// validateEmergencyEntryConfig parses subnets and URL templates up front and
// fails fast on malformed config.
func validateEmergencyEntryConfig(cfg *Config, path string) {
	e := cfg.EmergencyEntry
	if e == nil {
		return
	}

	emergencyEntryNets = nil
	for _, c := range e.AllowedSubnets {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			log.Fatalf("error: emergency_entry.allowed_subnets contains invalid CIDR %q in %s: %v", c, path, err)
		}
		emergencyEntryNets = append(emergencyEntryNets, n)
	}
	if len(emergencyEntryNets) == 0 {
		log.Printf("warning: emergency_entry.allowed_subnets is empty in %s; nobody can use emergency entry", path)
	}
	if cfg.Oidc == nil || cfg.Oidc.MembershipExpirationTimestampClaim == "" {
		log.Printf("warning: oidc.membership_expiration_timestamp_claim is not set in %s; emergency entry will not check membership expiration", path)
	}

	emergencyEntryTemplates = map[string]*template.Template{}
	for i := range e.Doors {
		door := &e.Doors[i]
		if door.ID == "" {
			log.Fatalf("error: emergency_entry.doors[%d] has empty id in %s", i, path)
		}
		if _, dup := emergencyEntryTemplates[door.ID]; dup {
			log.Fatalf("error: duplicate emergency_entry door id %q in %s", door.ID, path)
		}
		door.Method = strings.ToUpper(door.Method)
		if door.Method == "" {
			door.Method = http.MethodPost
		}
		tmpl, err := template.New(door.ID).Option("missingkey=error").Parse(door.URL)
		if err != nil {
			log.Fatalf("error: emergency_entry.doors[%d] (%s) has invalid url template in %s: %v", i, door.ID, path, err)
		}
		emergencyEntryTemplates[door.ID] = tmpl
	}
}

type emergencyEntryDoorResponse struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	LocalizedName LocalizedString `json:"localized_name,omitempty"`
}

type emergencyEntryStatusResponse struct {
	Status string                       `json:"status"`
	Doors  []emergencyEntryDoorResponse `json:"doors"`
}

// emergencyEntryAccess runs the subnet, session, group and membership checks.
// With fresh=true the user's claims are re-fetched from the IdP instead of
// using the session's cached copy. It returns the access status and, when the
// status is ok, the username.
func emergencyEntryAccess(c *fiber.Ctx, fresh bool) (string, string) {
	cfg := ConfigInstance.EmergencyEntry
	if cfg == nil {
		return emergencyEntryDisabled, ""
	}

	ip := net.ParseIP(c.IP())
	if ip == nil || !ipInAny(ip, emergencyEntryNets) {
		return emergencyEntryOutsideNetwork, ""
	}

	cookie := c.Cookies(CookieName)
	if cookie == "" {
		return emergencyEntryNotLoggedIn, ""
	}
	var session SessionModel
	if err := gormDB.First(&session, "id = ?", cookie).Error; err != nil {
		return emergencyEntryNotLoggedIn, ""
	}
	// Kiosk tablets have no user identity and may never open doors.
	if session.IsTablet {
		return emergencyEntryForbidden, ""
	}

	var claims map[string]interface{}
	if fresh {
		var err error
		claims, err = refreshSessionClaims(&session)
		if err != nil {
			log.Printf("Emergency entry: failed to refresh claims for %s: %v", session.Username, err)
			return emergencyEntryNotLoggedIn, ""
		}
	} else if err := json.Unmarshal([]byte(session.CachedClaims), &claims); err != nil {
		return emergencyEntryNotLoggedIn, ""
	}

	if len(cfg.RequiredGroups) > 0 {
		groups := groupsFromClaims(claims)
		if !slices.ContainsFunc(cfg.RequiredGroups, func(g string) bool { return slices.Contains(groups, g) }) {
			return emergencyEntryForbidden, ""
		}
	}

	if !membershipActive(claims) {
		return emergencyEntryMembershipExpired, ""
	}

	username, _ := extractUserInfo(claims)["username"].(string)
	if username == "" {
		username = session.Username
	}
	return emergencyEntryOK, username
}

// membershipActive reports whether the membership expiration claim lies in the
// future. When no claim is configured the check is skipped; when it is
// configured but missing or unparsable the membership counts as expired.
func membershipActive(claims map[string]interface{}) bool {
	claimName := ""
	if ConfigInstance.Oidc != nil {
		claimName = ConfigInstance.Oidc.MembershipExpirationTimestampClaim
	}
	if claimName == "" {
		return true
	}

	var expiresAt float64
	switch v := claims[claimName].(type) {
	case float64:
		expiresAt = v
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
		expiresAt = f
	default:
		return false
	}
	return time.Unix(int64(expiresAt), 0).After(time.Now())
}

// handleEmergencyEntryStatus reports whether the caller may use emergency entry
// and, if so, which doors are available.
func handleEmergencyEntryStatus(c *fiber.Ctx) error {
	status, _ := emergencyEntryAccess(c, false)
	resp := emergencyEntryStatusResponse{Status: status, Doors: []emergencyEntryDoorResponse{}}
	if status == emergencyEntryOK {
		for _, d := range ConfigInstance.EmergencyEntry.Doors {
			resp.Doors = append(resp.Doors, emergencyEntryDoorResponse{
				ID:            d.ID,
				Name:          d.Name,
				LocalizedName: d.LocalizedName,
			})
		}
	}
	return c.JSON(resp)
}

type emergencyEntryOpenRequest struct {
	ID string `json:"id"`
}

// handleEmergencyEntryOpen re-validates the caller against fresh IdP claims and
// fires the configured HTTP request for the requested door.
func handleEmergencyEntryOpen(c *fiber.Ctx) error {
	var req emergencyEntryOpenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	status, username := emergencyEntryAccess(c, true)
	if status != emergencyEntryOK {
		code := fiber.StatusForbidden
		switch status {
		case emergencyEntryDisabled:
			code = fiber.StatusServiceUnavailable
		case emergencyEntryNotLoggedIn:
			code = fiber.StatusUnauthorized
		}
		log.Printf("Emergency entry denied for door %q from %s: %s", req.ID, c.IP(), status)
		return c.Status(code).JSON(fiber.Map{"status": status, "error": status})
	}

	idx := slices.IndexFunc(ConfigInstance.EmergencyEntry.Doors, func(d EmergencyEntryDoor) bool { return d.ID == req.ID })
	if idx < 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Unknown door"})
	}
	door := ConfigInstance.EmergencyEntry.Doors[idx]

	log.Printf("Emergency entry: user %s opening door %s from %s", username, door.ID, c.IP())
	if err := fireEmergencyEntryRequest(door, username); err != nil {
		log.Printf("Emergency entry: opening door %s for %s failed: %v", door.ID, username, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func fireEmergencyEntryRequest(door EmergencyEntryDoor, username string) error {
	var url bytes.Buffer
	if err := emergencyEntryTemplates[door.ID].Execute(&url, struct{ Username string }{username}); err != nil {
		return fmt.Errorf("rendering url: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), emergencyEntryRequestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, door.Method, url.String(), nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		// The error embeds the URL, which may carry secrets: log it, but only
		// return a generic message to the client.
		log.Printf("Emergency entry: request for door %s failed: %v", door.ID, err)
		var urlErr interface{ Timeout() bool }
		if errors.As(err, &urlErr) && urlErr.Timeout() {
			return errors.New("door controller timed out")
		}
		return errors.New("door controller unreachable")
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("door controller responded with HTTP %d", resp.StatusCode)
	}
	return nil
}
