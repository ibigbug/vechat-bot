package queue

import (
	"fmt"
)

type MessageType int

const (
	TypeWechat MessageType = iota
	TypeTelegram
	TypeShutdown
)

type Message struct {
	FromType  MessageType
	ToType    MessageType
	FromMsgId string
	ToMsgId   string
	FromUser  string
	ToUser    string
	Content   string
	Extra     interface{}
}

func (m *Message) String() string {
	return fmt.Sprintf("queued message: %s<%d> -> %s<%d>", m.FromUser, m.FromType, m.ToUser, m.ToType)
}

type Terminal interface {
	Notify(msg *Message) error
}
