package service

import (
	"github.com/u16-io/FindSenryu4Discord/db"
	"github.com/u16-io/FindSenryu4Discord/model"
)

// IsOptOut checks if the user has opted out of being selected as an avatar candidate
func IsOptOut(userID string) bool {
	var count int
	if err := db.DB.Model(&model.OptOut{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// ToggleOptOut toggles the opt-out status for the user
// Returns true if the user is now opted out, false if opted in
func ToggleOptOut(userID string) (bool, error) {
	if IsOptOut(userID) {
		// Currently opted out, remove from DB (Opt-in)
		err := db.DB.Where("user_id = ?", userID).Delete(&model.OptOut{}).Error
		return false, err
	}
	// Currently opted in, add to DB (Opt-out)
	optout := model.OptOut{UserID: userID}
	err := db.DB.Create(&optout).Error
	return true, err
}
