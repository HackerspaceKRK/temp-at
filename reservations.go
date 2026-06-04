package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ReservationEvent is the normalized calendar event sent to the tablet.
type ReservationEvent struct {
	ID          int    `json:"id"`
	PHID        string `json:"phid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsAllDay    bool   `json:"is_all_day"`
	// Start and End are Unix epoch seconds.
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
	Timezone string `json:"timezone"`
	URL      string `json:"url"`
	// CreatedBy is the display name of the event creator. Currently always
	// empty: the Conduit calendar.event.search API does not expose the
	// host/creator for our token (not in fields, transactions, subscribers or
	// phid.query).
	// TODO: populate once a token/endpoint that exposes the creator is available.
	CreatedBy string `json:"created_by"`
}

// conduitResponse is the standard Phabricator Conduit envelope.
type conduitResponse struct {
	Result    json.RawMessage `json:"result"`
	ErrorCode *string         `json:"error_code"`
	ErrorInfo *string         `json:"error_info"`
}

// conduitCall performs a Conduit API call against the configured Phabricator
// instance and returns the raw `result` payload.
func conduitCall(method string, form url.Values) (json.RawMessage, error) {
	base := strings.TrimRight(ConfigInstance.Phabricator.URL, "/")
	if base == "" {
		return nil, fmt.Errorf("phabricator.url is not configured")
	}
	token := ConfigInstance.Phabricator.APIToken
	if token == "" {
		return nil, fmt.Errorf("phabricator.api_token is not configured")
	}

	form.Set("api.token", token)

	endpoint := base + "/api/" + method
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope conduitResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode conduit response: %w", err)
	}
	if envelope.ErrorCode != nil {
		info := ""
		if envelope.ErrorInfo != nil {
			info = *envelope.ErrorInfo
		}
		return nil, fmt.Errorf("conduit error %s: %s", *envelope.ErrorCode, info)
	}
	return envelope.Result, nil
}

// conduitEventSearchResult mirrors the fields of calendar.event.search we use.
type conduitEventSearchResult struct {
	Data []struct {
		ID     int    `json:"id"`
		PHID   string `json:"phid"`
		Fields struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			IsAllDay      bool   `json:"isAllDay"`
			StartDateTime struct {
				Epoch    int64  `json:"epoch"`
				Timezone string `json:"timezone"`
			} `json:"startDateTime"`
			EndDateTime struct {
				Epoch int64 `json:"epoch"`
			} `json:"endDateTime"`
		} `json:"fields"`
	} `json:"data"`
}

var (
	reservationsCacheMutex sync.Mutex
	reservationsCache      []ReservationEvent
	reservationsCacheAt    time.Time
)

const reservationsCacheTTL = 30 * time.Second

// fetchReservations returns today's calendar events, cached briefly so that
// many kiosks polling at once do not hammer Phabricator.
func fetchReservations() ([]ReservationEvent, error) {
	reservationsCacheMutex.Lock()
	defer reservationsCacheMutex.Unlock()

	if reservationsCache != nil && time.Since(reservationsCacheAt) < reservationsCacheTTL {
		return reservationsCache, nil
	}

	form := url.Values{}
	// Builtin "day" query returns today's events. The arbitrary-range
	// constraints (rangeStart/rangeEnd) are not supported by this method.
	form.Set("queryKey", "day")

	raw, err := conduitCall("calendar.event.search", form)
	if err != nil {
		return nil, err
	}

	var parsed conduitEventSearchResult
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse event search result: %w", err)
	}

	base := strings.TrimRight(ConfigInstance.Phabricator.URL, "/")
	events := make([]ReservationEvent, 0, len(parsed.Data))
	for _, e := range parsed.Data {
		events = append(events, ReservationEvent{
			ID:          e.ID,
			PHID:        e.PHID,
			Name:        e.Fields.Name,
			Description: e.Fields.Description,
			IsAllDay:    e.Fields.IsAllDay,
			Start:       e.Fields.StartDateTime.Epoch,
			End:         e.Fields.EndDateTime.Epoch,
			Timezone:    e.Fields.StartDateTime.Timezone,
			URL:         fmt.Sprintf("%s/E%d", base, e.ID),
		})
	}

	reservationsCache = events
	reservationsCacheAt = time.Now()
	return events, nil
}

// handleReservations serves today's calendar reservations. It is registered
// behind TabletAuthMiddleware so only kiosk tablets can read it.
func handleReservations(c *fiber.Ctx) error {
	events, err := fetchReservations()
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(events)
}
