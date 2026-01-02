package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var conf *Config

// Config 全ての設定を格納
type Config struct {
	Discord struct {
		Token    string
		Playing  string
		ClientID string
	}
}

func init() {
	// .env ファイルが存在すれば読み込む（存在しなくても環境変数から読める）
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment variables")
	}

	conf = &Config{}
	conf.Discord.Token = os.Getenv("DISCORD_TOKEN")
	conf.Discord.Playing = os.Getenv("DISCORD_PLAYING")
	conf.Discord.ClientID = os.Getenv("DISCORD_CLIENT_ID")

	if conf.Discord.Token == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}
}

// GetConf is return config
func GetConf() *Config {
	return conf
}
