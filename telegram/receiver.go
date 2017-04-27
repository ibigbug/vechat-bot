package telegram

import (
	"fmt"

	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
)

type Consumer struct {
	Queue chan *queue.Message
}

func (c Consumer) String() string {
	return fmt.Sprintf("Telegram Consumer %p", &c)
}

func (c Consumer) Notify(msg *queue.Message) error {
	c.Queue <- msg
	return nil
}

func (c *Consumer) Start() {
	for {
		select {
		case msg := <-c.Queue:
			logger.Println("tg consumer got new message", msg)
			if msg.ToType == queue.TypeShutdown {
				logger.Println("tg queue shutdown")
			} else {
				bot, err := GetBotByName(msg.ToUser)
				if err != nil {
					logger.Println("Error getting bot", msg.ToUser, "error", err, "msg", msg)
				} else {

					rv, err := bot.SendMessage(SendMessage{
						Text: msg.FromUser + ": " + msg.Content,
					})
					if err != nil || rv == nil {
						logger.Println("Error sending msg", msg, "need retry")
						continue
					}
					var record models.Message
					if _, err := models.Engine.Model(&record).
						Where("wechat_msg_id = ?", msg.FromMsgId).
						Set("telegram_msg_id = ?", rv.MessageId).
						Set("telegram_chat_id = ?", bot.ChatId).Update(); err != nil {
						logger.Println("error setting tg info to msg", msg, "error", err)
					}
				}
			}
		}
	}
}
