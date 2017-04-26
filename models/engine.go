package models

import (
	"github.com/go-pg/pg"
	"github.com/ibigbug/vechat-bot/config"
)

var Engine = pg.Connect(&pg.Options{
	User:     config.DatabaseUser,
	Password: config.DatabasePass,
	Addr:     config.DatabaseHost + ":" + config.DatabasePort,
	Database: config.DatabaseName,
})
