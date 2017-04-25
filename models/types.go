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
	Name      string `sql:",type:varchar(300), notnull"`
	Token     string `sql:",type:varchar(300), notnull"`
	Status    int    // 1: online, 2: offline, 3: invalid

	BaseModel
}

type WechatCredential struct {
	Id        int
	AccountId string            `sql:",type:varchar(300), notnull"`
	Username  string            `sql:",type:varchar(300), notnull"`
	Cookies   map[string]string `sql:", notnull"`
	Status    int               // 1: online, 2: offline, 3: invalid

	BaseModel
}

type ChannelBinding struct {
	Id                 int
	AccountId          string `sql:",type:varchar(300), notnull"`
	TelegramBotId      int
	WechatCredentialId int
	Status             int // 1: enabled, 0, disable

	BaseModel
}
