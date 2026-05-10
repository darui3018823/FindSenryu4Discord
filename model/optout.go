package model

import "time"

// OptOut represents a user who opted out of MIQ avatar selection
type OptOut struct {
	UserID    string `gorm:"primary_key"`
	CreatedAt time.Time
}
