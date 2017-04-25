package models

import (
	"fmt"
	"time"

	"github.com/go-pg/pg/orm"
)

type BaseModel struct {
	Created time.Time
	Updated time.Time
}

func (b *BaseModel) BeforeInsert(db orm.DB) error {
	if b.Created.IsZero() {
		b.Created = time.Now().UTC()
	}
	return nil
}

func (b *BaseModel) BeforeUpdate(db orm.DB) error {
	b.Updated = time.Now().UTC()
	return nil
}

type GoogleAccount struct {
	Id            int
	Sub           string `json:"sub" sql:",notnull,type:varchar(300)"`
	Name          string `json:"name" sql:",type:varchar(300)"`
	GivenName     string `json:"given_name" sql:",type:varchar(300)"`
	FamilyName    string `json:"family_name" sql:",type:varchar(300)"`
	Profile       string `json:"profile" sql:",type:varchar(300)"`
	Picture       string `json:"picture" sql:",type:varchar(300)"`
	Email         string `json:"email" sql:",notnull,unique,type:varchar(300)"` // we use email as pk, though google use sub
	EmailVerified bool   `json:"email_verified"`
	Gender        string `json:"gender" sql:",type:varchar(10)"`

	AccessToken  string `json:"-" sql:",type:varchar(300)"`
	RefreshToken string `json:"-" sql:",type:varchar(300)"`

	BaseModel
}

func (a GoogleAccount) String() string {
	return fmt.Sprintf("GoogleAccount{Id: %d, Email: %s}", a.Id, a.Email)
}

type TelegramBot struct {
	Id        int
	AccountId string `sql:",type:varchar(300), notnull"`
	Name      string `sql:",type:varchar(300), notnull, unique"`
	Token     string `sql:",type:varchar(300), notnull"`
	Status    int    // 1: online, 2: offline, 3: invalid
	ChatId    int64

	BaseModel
}

func (t TelegramBot) String() string {
	return fmt.Sprintf("TelegramBot{Id: %d, Name: %s, ChatId: %d}", t.Id, t.Name, t.ChatId)
}

// TODO store credential to make
// auto-reconnect for server restart
type WechatCredential struct {
	Id         int
	AccountId  string            `sql:",type:varchar(300), notnull"`
	Username   string            `sql:",type:varchar(300), notnull"`
	Cookies    map[string]string `sql:", notnull"`
	PassTicket string            `sql:",type:varchar(300)"`
	Sid        string            `sql:",type:varchar(300)"`
	Skey       string            `sql:",type:varchar(300)"`
	SyncKey    []string
	Status     int // 1: online, 2: offline, 3: invalid

	BaseModel
}

type Message struct {
	Id                 int
	WechatMsgId        string `sql:",type:varchar(300), notnull"`
	WechatFromUser     string `sql:",type:varchar(300), notnull"`
	WechatToUser       string `sql:",type:varchar(300), notnull"`
	WechatFromNickName string `sql:",type:varchar(300)"`
	WechatToNickName   string `sql:",type:varchar(300)"`

	TelegramChatId int64 `sql:",notnull"`
	TelegramMsgId  int64

	Content string `sql:",type:text, notnull"`

	BaseModel
}
