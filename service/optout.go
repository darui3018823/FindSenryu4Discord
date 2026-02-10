package service

import (
	"github.com/u16-io/FindSenryu4Discord/db"
)

const optOutKey = "miq_optout"

// IsOptOut checks if the user has opted out of being selected as an avatar candidate
func IsOptOut(userID string) bool {
	if db.LDB == nil {
		return false
	}
	// Check if userID is in the set
	n, err := db.LDB.SIsMember([]byte(optOutKey), []byte(userID))
	if err != nil {
		return false
	}
	return n == 1
}

// ToggleOptOut toggles the opt-out status for the user
// Returns true if the user is now opted out, false if opted in
func ToggleOptOut(userID string) (bool, error) {
	if db.LDB == nil {
		return false, nil // Should not happen if db is init
	}

	isMember, err := db.LDB.SIsMember([]byte(optOutKey), []byte(userID))
	if err != nil {
		return false, err
	}

	if isMember == 1 {
		// Currently opted out, remove from set (Opt-in)
		_, err := db.LDB.SRem([]byte(optOutKey), []byte(userID))
		if err != nil {
			return true, err // Still opted out on error
		}
		return false, nil
	} else {
		// Currently opted in, add to set (Opt-out)
		_, err := db.LDB.SAdd([]byte(optOutKey), []byte(userID))
		if err != nil {
			return false, err // Still opted in on error
		}
		return true, nil
	}
}
