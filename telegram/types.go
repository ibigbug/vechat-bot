package telegram

import (
	"fmt"
	"time"
)

type BaseResponse struct {
	Ok bool `json:"ok"`
}
type User struct {
	Id        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}
type UserResponse struct {
	BaseResponse
	Result User `json:"result"`
}

type Message struct {
	MessageId int    `json:"message_id"`
	From      User   `Json:"from"`
	Date      int    `json:"date"`
	Text      string `json:"text"`
}

func (m Message) String() string {
	return fmt.Sprintf("[%d] %s: %s %s", m.MessageId, m.From.FirstName, m.Text, time.Unix(int64(m.Date), 0))
}

type Update struct {
	UpdateId int     `json:"update_id"`
	Message  Message `json:"message"`
}

type UpdateResponse struct {
	BaseResponse
	Result []Update `json:"result"`
}
