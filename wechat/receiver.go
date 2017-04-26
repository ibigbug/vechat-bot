package wechat

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
			log.Println("wx consumer got new message", msg)
			if msg.ToType == queue.TypeShutdown {
				log.Println("wx queue shutdown")
			} else {
				bot, err := GetByUserName(msg.ToUser)
				if err != nil {
					log.Println("Error getting bot", msg.ToUser, "error", err, "msg", msg)
				} else {

					err := bot.SendMessage(SendMessage{
						FromUserName: msg.ToUser,
						ToUserName:   msg.Extra.(map[string]string)["TargetFriend"],
						Content:      msg.FromUser + ": " + msg.Content,
					})
					if err != nil {
						log.Println("Error sending msg", msg, "need retry")
						continue
					}
				}
			}
		}
	}
}
