package main

import "github.com/ibigbug/vechat-bot/models"
import "github.com/go-pg/pg/orm"
import "fmt"

func createTable() {
	tables := []interface{}{
		new(models.GoogleAccount),
		new(models.TelegramBot),
		new(models.WechatCredential),
		new(models.ChannelBinding),
	}
	for _, t := range tables {
		err := models.Engine.CreateTable(t, &orm.CreateTableOptions{
			IfNotExists: true,
		})
		fmt.Println(err)
	}
}
