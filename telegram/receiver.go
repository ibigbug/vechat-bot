package telegram

import (
	"log"

	"github.com/ibigbug/vechat-bot/models"
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
				bot, err := GetBotByName(msg.ToUser)
				if err != nil {
					log.Println("Error getting bot", msg.ToUser, "error", err, "msg", msg)
				} else {

					rv, err := bot.SendMessage(SendMessage{
						Text: msg.FromUser + ": " + msg.Content,
					})
					if err != nil {
						log.Println("Error sending msg", msg, "need retry")
						continue
					}
					var record models.Message
					if _, err := models.Engine.Model(&record).
						Where("wechat_msg_id = ?", msg.FromMsgId).
						Set("telegram_msg_id = ?", rv.MessageId).
						Set("telegram_chat_id = ?", bot.ChatId).Update(); err != nil {
						log.Println("error setting tg info to msg", msg, "error", err)
					}
				}
			}
		}
	}
}
