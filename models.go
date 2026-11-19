package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

// VirtualDeviceModel is the persistent record for a virtual device.
type VirtualDeviceModel struct {
	ID   uint   `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"uniqueIndex;not null"`
	Type string `gorm:"not null;index"`
	// Records the last time we have seen any indication that this device is working
	LastSeenAt *time.Time `gorm:"index"`
}

// TableName overrides the default table name.
func (VirtualDeviceModel) TableName() string {
	return "virtual_device_models"
}

// VirtualDeviceStateModel records a state change for a virtual device (timeseries data).
type VirtualDeviceStateModel struct {
	Timestamp       int64              `gorm:"index:idx_device_timestamp;not null"`
	VirtualDeviceID uint               `gorm:"index:idx_device_timestamp;not null"`
	VirtualDevice   VirtualDeviceModel `gorm:"foreignKey:VirtualDeviceID"`
	State           string             `gorm:"type:text;not null"` // JSON-encoded state
}

// TableName overrides the default table name.
func (VirtualDeviceStateModel) TableName() string {
	return "virtual_device_state_models"
}

// SessionModel represents an authenticated user session.
type SessionModel struct {
	ID           string    `gorm:"primaryKey;type:text"` // Random UUID
	Subject      string    `gorm:"index;not null"`       // QIDC sub
	IdPSessionID string    `gorm:"index"`                // QIDC sid
	Username     string    `gorm:"not null"`             // Cached preferred_username
	AccessToken  string    `gorm:"type:text"`
	RefreshToken string    `gorm:"type:text"`
	CachedClaims string    `gorm:"type:text"` // JSON-encoded claims
	ExpiresAt    time.Time `gorm:"not null;index"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	// IsTablet marks a long-lived session granted to a trusted kiosk tablet.
	// Such sessions have no OIDC tokens and are not refreshed.
	IsTablet bool `gorm:"not null;default:false"`
}

// TableName overrides the default table name.
func (SessionModel) TableName() string {
	return "sessions"
}

// GenerateUUIDv7 generates a new UUIDv7 string (used for session IDs).
func GenerateUUIDv7() string {
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to timestamp-based ID if UUIDv7 fails
		return uuid.Must(uuid.NewV4()).String()
	}
	return id.String()
}

// UsageStatsDayCache stores pre-computed daily usage stats per room.
// RoomID is "" when the cache covers all rooms combined.
// Date is stored as "2006-01-02" in the server's local timezone.
type UsageStatsDayCache struct {
	ID          uint    `gorm:"primaryKey;autoIncrement"`
	RoomID      string  `gorm:"uniqueIndex:idx_cache_room_date;not null"`
	Date        string  `gorm:"uniqueIndex:idx_cache_room_date;not null"`
	MaxPeople   int     `gorm:"not null"`
	ManHours    float64 `gorm:"not null"`
	ActiveHours float64 `gorm:"not null"`
	HourlyData  string  `gorm:"type:text;not null"` // JSON-encoded []UsageHeatmapDataPoint (24 entries, one per hour)
}

func (UsageStatsDayCache) TableName() string {
	return "usage_stats_day_caches"
}

// NumericSeriesDayCache stores the bucket averages of one numeric series for a
// complete local calendar day at a given bucket interval.
type NumericSeriesDayCache struct {
	ID              uint   `gorm:"primaryKey;autoIncrement"`
	SeriesKey       string `gorm:"uniqueIndex:idx_numeric_series_day;not null"`
	Date            string `gorm:"uniqueIndex:idx_numeric_series_day;not null"`
	IntervalMinutes int    `gorm:"uniqueIndex:idx_numeric_series_day;not null"`
	Data            string `gorm:"type:text;not null"` // JSON []*float64
}

func (NumericSeriesDayCache) TableName() string {
	return "numeric_series_day_caches"
}

// DhcpLeaseModel is one tracked DHCP lease, keyed by MAC. Rows are updated in
// place on each scrape, so the table stays bounded by the number of distinct
// devices rather than growing over time.
type DhcpLeaseModel struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	MacAddress string `gorm:"uniqueIndex:idx_dhcp_mac;not null"` // normalized aa:bb:cc:dd:ee:ff
	Server     string // DHCP server name (informational; updated in place)
	IPAddress  string `gorm:"index;not null"`
	Hostname   string
	Comment    string
	Dynamic    bool
	// FirstSeen is the first scrape ever in which this MAC appeared. Immutable.
	FirstSeen int64 `gorm:"not null"`
	// LeaseStart marks the beginning of the current continuous online period.
	// It resets when the device reappears after being absent (the raw DHCP
	// lease start is useless here because the lease time is only minutes).
	LeaseStart int64 `gorm:"not null"`
	// LastSeen is the most recent scrape in which this MAC was bound.
	LastSeen int64 `gorm:"not null;index"`
}

func (DhcpLeaseModel) TableName() string {
	return "dhcp_leases"
}

// AppSettingModel is a simple key/value store for server-managed settings that
// must survive restarts (e.g. the generated VAPID keypair for web push).
type AppSettingModel struct {
	Key   string `gorm:"primaryKey;type:text"`
	Value string `gorm:"type:text;not null"`
}

func (AppSettingModel) TableName() string {
	return "app_settings"
}

type PushSubscriptionModel struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Endpoint  string `gorm:"uniqueIndex:idx_push_endpoint_printer;not null"`
	P256dh    string `gorm:"not null"`
	Auth      string `gorm:"not null"`
	PrinterID string `gorm:"uniqueIndex:idx_push_endpoint_printer;index;not null"`
	TaskID    string
	CreatedAt int64 `gorm:"not null"`
}

func (PushSubscriptionModel) TableName() string {
	return "push_subscriptions"
}

// PrinterThumbnailModel caches print preview PNGs, keyed by printer + task +
// start time. Bambu stores the plate preview extracted from a print's
// .gcode.3mf (fetched over FTPS at most once per print). Anycubic stores the
// slicer preview of every file on the printer, keyed by its "<name>/<internal
// name>" listing entry with GcodeStartTime 0, since previews cannot be fetched
// while printing. CreatedAt is retained so stale rows can be cleaned up later.
type PrinterThumbnailModel struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	PrinterID      string    `gorm:"uniqueIndex:idx_printer_thumb,priority:1;not null"`
	TaskName       string    `gorm:"uniqueIndex:idx_printer_thumb,priority:2;not null"`
	GcodeStartTime int64     `gorm:"uniqueIndex:idx_printer_thumb,priority:3;not null"`
	PNG            []byte    `gorm:"type:blob"`
	CreatedAt      time.Time `gorm:"index"`
}

func (PrinterThumbnailModel) TableName() string {
	return "printer_thumbnails"
}

// AutoMigrateModels runs GORM auto-migration for all models.
func AutoMigrateModels(db *gorm.DB) error {
	if err := migrateDropStateModelID(db); err != nil {
		return fmt.Errorf("dropping virtual_device_state_models.id: %w", err)
	}
	return db.AutoMigrate(&VirtualDeviceModel{}, &VirtualDeviceStateModel{}, &SessionModel{}, &UsageStatsDayCache{}, &NumericSeriesDayCache{}, &DhcpLeaseModel{}, &AppSettingModel{}, &PushSubscriptionModel{}, &PrinterThumbnailModel{})
}

// CurrentTimestampMillis returns current time as Unix milliseconds.
func CurrentTimestampMillis() int64 {
	return time.Now().UnixMilli()
}

// Delete the old id uuidv7 column from state model
func migrateDropStateModelID(db *gorm.DB) error {
	// Not Migrator().HasColumn: the SQLite driver implements it with a LIKE on the
	// CREATE statement, which would also match "virtual_device_id".
	var hasID int64
	if err := db.Raw("SELECT count(*) FROM pragma_table_info('virtual_device_state_models') WHERE name = 'id'").
		Scan(&hasID).Error; err != nil {
		return err
	}
	if hasID == 0 {
		return nil
	}
	const oldTable = "virtual_device_state_models_old"
	log.Printf("Migration: rebuilding virtual_device_state_models without the id column, this may take a while...")
	start := time.Now()
	err := db.Transaction(func(tx *gorm.DB) error {
		// Index names are global in SQLite, so drop it before the new table recreates it.
		if err := tx.Exec("DROP INDEX IF EXISTS idx_device_timestamp").Error; err != nil {
			return err
		}
		if err := tx.Exec("DROP TABLE IF EXISTS " + oldTable).Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE virtual_device_state_models RENAME TO " + oldTable).Error; err != nil {
			return err
		}
		if err := tx.Migrator().CreateTable(&VirtualDeviceStateModel{}); err != nil {
			return err
		}
		// UUIDv7 IDs sort by creation time, so ordering by id preserves insertion order.
		res := tx.Exec("INSERT INTO virtual_device_state_models (timestamp, virtual_device_id, state) " +
			"SELECT timestamp, virtual_device_id, state FROM " + oldTable + " ORDER BY id")
		if res.Error != nil {
			return res.Error
		}
		log.Printf("Migration: copied %d state rows", res.RowsAffected)
		return tx.Exec("DROP TABLE " + oldTable).Error
	})
	if err != nil {
		return err
	}
	log.Printf("Migration: table rebuilt in %s, running VACUUM...", time.Since(start).Round(time.Millisecond))
	start = time.Now()
	if err := db.Exec("VACUUM").Error; err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}
	log.Printf("Migration: VACUUM finished in %s", time.Since(start).Round(time.Millisecond))
	return nil
}
