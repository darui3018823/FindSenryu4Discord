package model

// YomeMessage stores the author IDs for "ここで一句" messages
type YomeMessage struct {
	MessageID string `gorm:"primaryKey"`
	Author1ID string // 上の句の詠み手
	Author2ID string // 中の句の詠み手
	Author3ID string // 下の句の詠み手
}
