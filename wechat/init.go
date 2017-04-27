package wechat

import (
	"log"

	"os"

	"github.com/ibigbug/vechat-bot/queue"
)

var logger = log.New(os.Stdout, "[wechat]", log.LstdFlags)

func init() {
	logger.Println("Surviving bots")
	var consumer = Consumer{
		Queue: make(chan *queue.Message),
	}
	queue.MessageSwitcher.Register(queue.TypeWechat, &consumer)
	go consumer.Start()
}
