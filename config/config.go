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
	DatabaseHost = Env("", "localhost")
	DatabaseUser = Env("", "admin")
	DatabasePass = Env("", "ali123")
	DatabaseName = Env("", "vechat-sync-dev")
)
