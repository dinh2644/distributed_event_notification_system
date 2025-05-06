package env

import (
	"log"
	"os"
)

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Printf("Using fallback for %s: %s", key, fallback)
	return fallback
}
