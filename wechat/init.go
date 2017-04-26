package wechat

import (
	"log"

	"github.com/ibigbug/vechat-bot/queue"
)

func init() {
	log.Println("Surviving bots")
	var consumer = Consumer{
		Queue: make(chan *queue.Message),
	}
	queue.MessageSwitcher.Register(queue.TypeWechat, &consumer)
	go consumer.Start()
}
