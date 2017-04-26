package main

import (
	"fmt"

	"github.com/go-pg/pg/orm"
	"github.com/ibigbug/vechat-bot/models"
)

func createTable() {
	tables := []interface{}{
		new(models.GoogleAccount),
		new(models.TelegramBot),
		new(models.WechatCredential),
		new(models.Message),
	}
	for _, t := range tables {
		err := models.Engine.CreateTable(t, &orm.CreateTableOptions{
			IfNotExists: true,
		})
		fmt.Println("sync db error: ", err)
	}
}
