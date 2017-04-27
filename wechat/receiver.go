package wechat

import (
	"fmt"

	"github.com/ibigbug/vechat-bot/queue"
)

type Consumer struct {
	Queue chan *queue.Message
}

func (c Consumer) String() string {
	return fmt.Sprintf("Wechat Consumer %p", &c)
}

func (c Consumer) Notify(msg *queue.Message) error {
	c.Queue <- msg
	return nil
}

func (c *Consumer) Start() {
	for {
		select {
		case msg := <-c.Queue:
			logger.Println("wx consumer got new message", msg)
			if msg.ToType == queue.TypeShutdown {
				logger.Println("wx queue shutdown")
			} else {
				bot, err := GetByUserName(msg.ToUser)
				if err != nil {
					logger.Println("Error getting bot", msg.ToUser, "error", err, "msg", msg)
				} else {

					err := bot.SendMessage(SendMessage{
						FromUserName: msg.ToUser,
						ToUserName:   msg.Extra.(map[string]string)["TargetFriend"],
						Content:      msg.FromUser + ": " + msg.Content,
					})
					if err != nil {
						logger.Println("Error sending msg", msg, "need retry")
						continue
					}
				}
			}
		}
	}
}
