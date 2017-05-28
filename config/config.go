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
	ServerAddr        = ":" + Env("PORT", "5000")
	GoogleCallbackURL = Env("GOOGLE_CALLBACK_URL", "http://dev:5000/account/callback")

	DatabaseHost = Env("DATABASE_HOST", "localhost")
	DatabasePort = Env("DATABASE_PORT", "5432")
	DatabaseUser = Env("DATABASE_USER", "admin")
	DatabasePass = Env("DATABASE_PASS", "pass")
	DatabaseName = Env("DATABASE_NAME", "vechat-sync-dev")

	SentryDSN = Env("SENTRY_DSN", "")

	SurviveTelegramBots = Env("SURVIVE_TELEGRAM_BOTS", "false")
	SurviveWechatBots   = Env("SURVIVE_WECHAT_BOTS", "false")
)
