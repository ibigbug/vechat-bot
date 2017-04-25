package models

import (
	"github.com/go-pg/pg"
)

var Engine = pg.Connect(&pg.Options{
	User:     "admin",
	Password: "ali123",
	Addr:     ":32768",
	Database: "vechat-sync-dev",
})
