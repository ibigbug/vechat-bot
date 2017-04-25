package models

import "fmt"

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
}

func (a GoogleAccount) String() string {
	return fmt.Sprintf("GoogleAccount{Id: %d, Email: %s}", a.Id, a.Email)
}

type ChannelBinding struct {
	Id               int
	AccountId        int
	TelegramBot      string `sql:",type:varchar(300),notnull"`
	TelegramToken    string `sql:",type:varchar(300),notnull"`
	WechatUsername   string `sql:",type:varchar(300),notnull"`
	WechatCredential string `sql:",type:text,notnull"`
}
