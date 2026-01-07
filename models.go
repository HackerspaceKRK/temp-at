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

// AutoMigrateModels runs GORM auto-migration for all models.
func AutoMigrateModels(db *gorm.DB) error {
	return db.AutoMigrate(&VirtualDeviceModel{}, &VirtualDeviceStateModel{}, &SessionModel{})
}

// CurrentTimestampMillis returns current time as Unix milliseconds.
func CurrentTimestampMillis() int64 {
	return time.Now().UnixMilli()
}
