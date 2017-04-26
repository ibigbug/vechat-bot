package queue

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

type Terminal interface {
	Notify(msg *Message) error
}
