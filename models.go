package main

import (
	"time"

	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
)

// VirtualDeviceModel is the persistent record for a virtual device.
type VirtualDeviceModel struct {
	ID   uint   `gorm:"primaryKey;autoIncrement"`
	Name string `gorm:"uniqueIndex;not null"`
	Type string `gorm:"not null;index"`
}

// TableName overrides the default table name.
func (VirtualDeviceModel) TableName() string {
	return "virtual_device_models"
}

// VirtualDeviceStateModel records a state change for a virtual device (timeseries data).
type VirtualDeviceStateModel struct {
	ID              string             `gorm:"primaryKey;type:text"` // UUIDv7 as string
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

// GenerateUUIDv7 generates a new UUIDv7 string for state record IDs.
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

// AutoMigrateModels runs GORM auto-migration for all models.
func AutoMigrateModels(db *gorm.DB) error {
	return db.AutoMigrate(&VirtualDeviceModel{}, &VirtualDeviceStateModel{}, &SessionModel{}, &UsageStatsDayCache{}, &DhcpLeaseModel{})
}

// CurrentTimestampMillis returns current time as Unix milliseconds.
func CurrentTimestampMillis() int64 {
	return time.Now().UnixMilli()
}
