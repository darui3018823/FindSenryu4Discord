package db

import (
	"log"
	"os"

	"github.com/jinzhu/gorm"
	lediscfg "github.com/ledisdb/ledisdb/config"
	"github.com/ledisdb/ledisdb/ledis"
	"github.com/u16-io/FindSenryu4Discord/model"

	// SQLite3 driver for Gorm
	_ "github.com/mattn/go-sqlite3"
)

var (
	// DB is GormDB
	DB  *gorm.DB
	err error
)

// Init is initialize dbs from main function
func Init() {
	_, err := os.Stat("data")
	if os.IsNotExist(err) {
		if err := os.Mkdir("data", 0777); err != nil {
			log.Fatal(err)
		}
	}
	initDB()

	// Migrate data from LedisDB to SQLite if LedisDB exists
	_, err = os.Stat("data/ledis")
	if !os.IsNotExist(err) {
		log.Println("Found old LedisDB data. Migrating to SQLite...")
		cfg := lediscfg.NewConfigDefault()
		cfg.DataDir = "data/ledis"
		l, err := ledis.Open(cfg)
		if err != nil {
			log.Printf("Warning: Failed to open LedisDB for migration: %v", err)
			return
		}
		LDB, err := l.Select(0)
		if err != nil {
			log.Printf("Warning: Failed to select LedisDB DB for migration: %v", err)
			return
		}

		// Migrate Mutes
		mutes, err := LDB.SMembers([]byte("mute"))
		if err == nil {
			for _, m := range mutes {
				channelID := string(m)
				var count int
				DB.Model(&model.Mute{}).Where("channel_id = ?", channelID).Count(&count)
				if count == 0 {
					DB.Create(&model.Mute{ChannelID: channelID})
				}
			}
		}

		// Migrate OptOuts
		optouts, err := LDB.SMembers([]byte("miq_optout"))
		if err == nil {
			for _, o := range optouts {
				userID := string(o)
				var count int
				DB.Model(&model.OptOut{}).Where("user_id = ?", userID).Count(&count)
				if count == 0 {
					DB.Create(&model.OptOut{UserID: userID})
				}
			}
		}

		l.Close()

		// Remove old LedisDB directory
		err = os.RemoveAll("data/ledis")
		if err != nil {
			log.Printf("Warning: Failed to delete old LedisDB directory: %v", err)
		} else {
			log.Println("LedisDB migration complete. Old data deleted.")
		}
	}
}

func initDB() {
	DB, err = gorm.Open("sqlite3", "data/senryu.db")
	if err != nil {
		panic(err)
	}
	DB.AutoMigrate(&model.Senryu{}, &model.YomeMessage{}, &model.AvatarCache{}, &model.Mute{}, &model.OptOut{})
}

// Close is closing db
func Close() {
	if err := DB.Close(); err != nil {
		panic(err)
	}
}
