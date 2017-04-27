package wechat

import (
	"encoding/xml"
	"fmt"
)

// SendMessage is a wechat.SendMessage sendable
type SendMessage struct {
	FromUserName string
	ToUserName   string
	Content      string
	MsgId        string
}

type SyncKey struct {
	Count int
	List  []struct {
		Key int
		Val int
	}
}

func (s SyncKey) GetValue() []string {
	var syncKey = make([]string, s.Count)
	for _, sk := range s.List {
		syncKey = append(syncKey, fmt.Sprintf("%d_%d", sk.Key, sk.Val))
	}
	return syncKey[len(syncKey)-s.Count:]
}

type WechatCredential struct {
	PassTicket string
	Sid        string
	Skey       string
	Uin        string
	SyncKey    SyncKey
}

type InitRequest struct {
	BaseRequest *BaseRequest
}

type requestMessage struct {
	ClientMsgId  string
	Content      string
	FromUserName string
	LocalID      string
	ToUserName   string
	Type         int
}
type SendMessageRequest struct {
	BaseRequest *BaseRequest
	Msg         requestMessage
	Scene       int
}

type BaseRequest struct {
	DeviceID string
	Sid      string
	Skey     string
	Uin      string
}

type BaseResponse struct {
	ErrMsg string
	Ret    int
}

type WechatFriend struct {
	Alias            string
	AppAccountFlag   int
	AttrStatus       int
	ChatRoomId       int
	City             string
	ContactFlag      int
	DisplayName      string
	EncryChatRoomId  string
	HeadImgUrl       string
	HideInputBarFlag int
	IsOwner          int
	KeyWord          string
	MemberCount      int
	MemberList       []*WechatFriend
	NickName         string
	OwnerUid         int
	PYInitial        string
	PYQuanPin        string
	Province         string
	RemarkName       string
	RemarkPYInitial  string
	RemarkPYQuanPin  string
	Sex              int
	Signature        string
	SnsFlag          int
	StarFriend       int
	Statues          int
	Uin              int
	UniFriend        int
	UserName         string
	VerifyFlag       int
}

type WechatUser struct {
	AppAccountFlag    int
	ContactFlag       int
	HeadImgFlag       int
	HeadImgUrl        string
	HideInputBarFlag  int
	NickName          string
	PYInitial         string
	PYQuanPin         string
	RemarkName        string
	RemarkPYInitial   string
	RemarkPYQuanPin   string
	Sex               int
	Signature         string
	SnsFlag           int
	StarFriend        int
	Uin               int
	UserName          string
	VerifyFlag        int
	WebWxPluginSwitch int
}

type InitResponse struct {
	BaseResponse        *BaseResponse
	ChatSet             string
	ClickReportInterval int
	ClientVersion       int
	ContactList         []*WechatFriend
	Count               int
	GrayScale           int
	InviteStartCount    int
	MPSubscribeMsgCount int
	MPSubscribeMsgList  []*struct {
		MPArticleCount int
		MPArticleList  []*struct {
			Cover  string
			Digest string
			Title  string
			Url    string
		}
		NickName string
		Time     int
		UserName string
	}
	Skey    string
	SyncKey struct {
		Count int
		List  []struct {
			Key int
			Val int
		}
	}
	SystemTime int
	User       *WechatUser
}

type LogonResponse struct {
	XMLName     xml.Name `xml:"error"`
	Ret         string   `xml:"ret"`
	Message     string   `xml:"message"`
	Skey        string   `xml:"skey"`
	Wxsid       string   `xml:"wxsid"`
	Wxuin       string   `xml:"wxuin"`
	PassTicket  string   `xml:"pass_ticket"`
	IsGrayScale string   `xml:"isgrayscale"`
}

type WebwxSyncResponse struct {
	AddMsgCount int
	AddMsgList  []*struct {
		Content      string
		FromUserName string
		MsgId        string
		NewMsgId     int64
		ToUserName   string
	}
	SyncKey SyncKey
}
