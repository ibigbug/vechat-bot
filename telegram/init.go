package telegram

import (
	"log"

	"os"

	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
)

var logger = log.New(os.Stdout, "[telegram]", log.LstdFlags)

func init() {

	logger.Println("Surviving bots")
	var bots []models.TelegramBot
	if err := models.Engine.Model(&bots).Where("status = ?", 1).Select(); err == nil {
		logger.Printf("Got %d bots to survive\n", len(bots))
		for _, b := range bots {
			cli := GetBotClient(b.Token, b.Name)
			cli.ChatId = b.ChatId
			go cli.GetUpdates()
			logger.Printf("Surived a bot %s\n", b.Name)
		}
	} else {
		logger.Printf("error occured while surviving bots: %s\n", err.Error())
	}

	var consumer = Consumer{
		Queue: make(chan *queue.Message),
	}
	queue.MessageSwitcher.Register(queue.TypeTelegram, &consumer)
	go consumer.Start()
}
