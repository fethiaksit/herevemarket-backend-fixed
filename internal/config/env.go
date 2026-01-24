package config

import (
	"log"
	"os"
	"strconv"
)

type Env struct {
	MongoURI              string
	DBName                string
	JWTSecret             string
	AccessTokenTTLMinutes int
	RefreshTokenTTLDays   int
}

func LoadEnv() Env {
	accessTTL, _ := strconv.Atoi(getEnv("ACCESS_TOKEN_TTL", "20"))
	refreshTTL, _ := strconv.Atoi(getEnv("REFRESH_TOKEN_TTL", "7"))

	return Env{
		MongoURI:              getEnv("MONGO_URI", ""),
		DBName:                getEnv("DB_NAME", "heremarket"),
		JWTSecret:             getEnv("JWT_SECRET", ""),
		AccessTokenTTLMinutes: accessTTL,
		RefreshTokenTTLDays:   refreshTTL,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	if fallback == "" {
		log.Fatalf("ENV %s is required", key)
	}
	return fallback
}
