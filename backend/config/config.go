package config

import (
	"os"
)

type Config struct {
	ServerPort   string
	MySQLDSN     string
	RedisAddr    string
	RedisPwd     string
	RedisDB      int
	JWTSecret    string
	TranslateURL string
}

func Load() *Config {
	return &Config{
		ServerPort:   getEnv("SERVER_PORT", ":8080"),
		MySQLDSN:     getEnv("MYSQL_DSN", "root:123456@tcp(127.0.0.1:3306)/clipboard_sync?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:    getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPwd:     getEnv("REDIS_PWD", ""),
		RedisDB:      0,
		JWTSecret:    getEnv("JWT_SECRET", "clipboard-sync-secret-key-2024"),
		TranslateURL: getEnv("TRANSLATE_URL", "https://api.mock-translate.com/translate"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
