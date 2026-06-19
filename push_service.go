package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// vapidSubscriber is the VAPID "sub" claim sent to push services. It should be a
// mailto: or https: URL identifying the application operator.
const vapidSubscriber = "mailto:info@hackerspace-krk.pl"

const vapidSettingKey = "vapid_keys"

type vapidKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// PushService manages Web Push (VAPID) keys and subscriptions and sends print
// notifications. Keys are generated once and persisted in the database so that
// existing browser subscriptions remain valid across restarts.
type PushService struct {
	db   *gorm.DB
	keys vapidKeys
	mu   sync.Mutex
}

// NewPushService loads the VAPID keypair from the database, generating and
// persisting one on first run.
func NewPushService(db *gorm.DB) (*PushService, error) {
	s := &PushService{db: db}
	if err := s.loadOrCreateKeys(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PushService) loadOrCreateKeys() error {
	var setting AppSettingModel
	err := s.db.Where("key = ?", vapidSettingKey).First(&setting).Error
	if err == nil {
		if jerr := json.Unmarshal([]byte(setting.Value), &s.keys); jerr == nil &&
			s.keys.Public != "" && s.keys.Private != "" {
			return nil
		}
		log.Printf("[push] stored VAPID keys invalid, regenerating")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	priv, pub, gerr := webpush.GenerateVAPIDKeys()
	if gerr != nil {
		return gerr
	}
	s.keys = vapidKeys{Public: pub, Private: priv}
	value, _ := json.Marshal(s.keys)
	if cerr := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&AppSettingModel{Key: vapidSettingKey, Value: string(value)}).Error; cerr != nil {
		return cerr
	}
	log.Printf("[push] generated new VAPID keypair")
	return nil
}

// PublicKey returns the base64url VAPID public key for the frontend.
func (s *PushService) PublicKey() string {
	return s.keys.Public
}

// Subscribe stores (or refreshes) a push subscription for a printer/print.
func (s *PushService) Subscribe(printerID, taskID, endpoint, p256dh, auth string) error {
	sub := PushSubscriptionModel{
		Endpoint:  endpoint,
		P256dh:    p256dh,
		Auth:      auth,
		PrinterID: printerID,
		TaskID:    taskID,
		CreatedAt: CurrentTimestampMillis(),
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "endpoint"}, {Name: "printer_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"p256dh", "auth", "task_id", "created_at"}),
	}).Create(&sub).Error
}

// Unsubscribe removes a subscription by its endpoint.
func (s *PushService) Unsubscribe(endpoint string) error {
	return s.db.Where("endpoint = ?", endpoint).Delete(&PushSubscriptionModel{}).Error
}

// SendPrintNotification delivers a notification to every subscription registered
// for the given printer's current print, then deletes them (one-shot).
func (s *PushService) SendPrintNotification(printerID, taskID, title, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var subs []PushSubscriptionModel
	q := s.db.Where("printer_id = ?", printerID)
	if taskID != "" {
		// Match the specific print, or subscriptions whose task was unknown.
		q = q.Where("task_id = ? OR task_id = ''", taskID)
	}
	if err := q.Find(&subs).Error; err != nil {
		log.Printf("[push] failed to load subscriptions for %s: %v", printerID, err)
		return
	}
	if len(subs) == 0 {
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
		"tag":   "bambu-" + printerID,
	})

	for _, sub := range subs {
		wp := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
		}
		resp, err := webpush.SendNotification(payload, wp, &webpush.Options{
			Subscriber:      vapidSubscriber,
			VAPIDPublicKey:  s.keys.Public,
			VAPIDPrivateKey: s.keys.Private,
			TTL:             60,
		})
		if err != nil {
			log.Printf("[push] send failed for %s: %v", sub.Endpoint, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			// Subscription expired/invalid; drop it.
			s.db.Where("id = ?", sub.ID).Delete(&PushSubscriptionModel{})
		}
	}

	// Clear the (now-fired) subscriptions for this print.
	ids := make([]uint, len(subs))
	for i, sub := range subs {
		ids[i] = sub.ID
	}
	if err := s.db.Where("id IN ?", ids).Delete(&PushSubscriptionModel{}).Error; err != nil {
		log.Printf("[push] failed to clear subscriptions: %v", err)
	}
}
