package model

import "time"

// Mute represents a muted channel
type Mute struct {
	ChannelID string `gorm:"primary_key"`
	CreatedAt time.Time
}
