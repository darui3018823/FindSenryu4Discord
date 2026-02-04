package model

import "time"

// AvatarCache stores the Discord avatar URL for a user
type AvatarCache struct {
	UserID    string    `gorm:"primaryKey"`
	AvatarURL string    `gorm:"not null"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
