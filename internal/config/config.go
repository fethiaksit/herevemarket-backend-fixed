package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var AppEnv Config

type Config struct {
	MongoURI        string
	DBName          string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not loaded:", err)
	}
	AppEnv = Config{
		MongoURI:        getEnvOrDefault("MONGO_URI", ""),
		DBName:          getEnvOrDefault("DB_NAME", "heremarket"),
		JWTSecret:       getEnvOrDefault("JWT_SECRET", ""),
		AccessTokenTTL:  getDurationEnv("ACCESS_TOKEN_TTL", 20, time.Minute),
		RefreshTokenTTL: getDurationEnv("REFRESH_TOKEN_TTL", 7, 24*time.Hour),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue int, unit time.Duration) time.Duration {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return time.Duration(parsed) * unit
		}
	}
	return time.Duration(defaultValue) * unit
}
