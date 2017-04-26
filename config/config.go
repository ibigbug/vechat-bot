package config

import (
	"os"
)

func Env(key, defaultValue string) string {
	if rv := os.Getenv(key); rv != "" {
		return rv
	}
	return defaultValue
}

var (
	DatabaseHost = Env("DATABASE_HOST", "localhost")
	DatabasePort = Env("DATABASE_PORT", "32768")
	DatabaseUser = Env("DATABASE_USER", "admin")
	DatabasePass = Env("DATABASE_PASS", "pass")
	DatabaseName = Env("DATABASE_NAME", "vechat-sync-dev")
)
