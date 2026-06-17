package service

import (
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
)

// IsMute is true if the channel is muted.
func IsMute(id string) bool {
	var count int
	if err := db.DB.Model(&model.Mute{}).Where("channel_id = ?", id).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// ToMute is to mute.
func ToMute(id string) error {
	if IsMute(id) {
		return nil
	}
	mute := model.Mute{ChannelID: id}
	return db.DB.Create(&mute).Error
}

// ToUnMute is to unmute.
func ToUnMute(id string) error {
	return db.DB.Where("channel_id = ?", id).Delete(&model.Mute{}).Error
}
