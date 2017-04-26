package telegram

import (
	"log"

	"github.com/ibigbug/vechat-bot/queue"
)

type Consumer struct {
	Queue chan *queue.Message
}

func (c Consumer) Notify(msg *queue.Message) error {
	c.Queue <- msg
	return nil
}

func (c *Consumer) Start() {
	for {
		select {
		case msg := <-c.Queue:
			log.Println("tg consumer got new message", msg)
			if msg.ToType == queue.TypeShutdown {
				log.Println("tg queue shutdown")
			} else {
				log.Println("Send to tg...")
			}
		}
	}
}
