package wechat

import (
	"log"

	"os"

	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
)

var logger = log.New(os.Stdout, "[wechat]", log.LstdFlags)

func init() {

	logger.Println("Surviving bots")
	var bots []models.WechatCredential
	if err := models.Engine.Model(&bots).Where("status = ?", 1).Select(); err == nil {
		logger.Printf("Got %d bots to survive\n", len(bots))
		for _, b := range bots {
			cli := FromCredential(&b)
			go cli.StartSyncCheck()
			logger.Printf("Surived a bot %s\n", cli)
		}
	} else {
		logger.Printf("error occured while surviving bots: %s\n", err.Error())
	}

	var consumer = Consumer{
		Queue: make(chan *queue.Message),
	}
	queue.MessageSwitcher.Register(queue.TypeWechat, &consumer)
	go consumer.Start()
}
