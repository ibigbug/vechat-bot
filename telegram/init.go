package telegram

import (
	"log"

	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
)

func init() {
	log.Println("Surviving bots")
	var bots []models.TelegramBot
	if err := models.Engine.Model(&bots).Where("status = ?", 1).Select(); err == nil {
		log.Printf("Got %d bots to survive\n", len(bots))
		for _, b := range bots {
			cli := GetBotClient(b.Token, b.Name)
			cli.ChatId = b.ChatId
			go cli.GetUpdates()
			log.Printf("Surived a bot %s\n", b.Name)
		}
	} else {
		log.Printf("error occured while surviving bots: %s\n", err.Error())
	}

	var consumer = Consumer{
		Queue: make(chan *queue.Message),
	}
	queue.MessageSwitcher.Register(queue.TypeTelegram, &consumer)
	go consumer.Start()
}
