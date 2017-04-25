package main

import "github.com/ibigbug/vechat-bot/models"
import "github.com/go-pg/pg/orm"
import "fmt"

func createTable() {
	err := models.Engine.CreateTable(new(models.GoogleAccount), &orm.CreateTableOptions{
		IfNotExists: true,
	})
	fmt.Println(err)
}
