package service

import (
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
)

// SaveYomeMessage saves the author IDs for a "ここで一句" message
func SaveYomeMessage(messageID, author1ID, author2ID, author3ID string) error {
	ym := model.YomeMessage{
		MessageID: messageID,
		Author1ID: author1ID,
		Author2ID: author2ID,
		Author3ID: author3ID,
	}
	if err := db.DB.Create(&ym).Error; err != nil {
		return err
	}
	return nil
}

// GetYomeMessage retrieves author IDs for a given message ID
func GetYomeMessage(messageID string) (*model.YomeMessage, error) {
	var ym model.YomeMessage
	if err := db.DB.Where("message_id = ?", messageID).First(&ym).Error; err != nil {
		return nil, err
	}
	return &ym, nil
}
