package wechat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"errors"

	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
	"golang.org/x/net/publicsuffix"
)

const (
	BaseCookieURL      = "https://wxq.qq.com"
	GetUUIDURL         = "https://login.wx2.qq.com/jslogin?appid=wx782c26e4c19acffb&redirect_uri=https%3A%2F%2Fwx.qq.com%2Fcgi-bin%2Fmmwebwx-bin%2Fwebwxnewloginpage&fun=new&lang=en_US&_=1492959953169"
	CheckLoginURL      = "https://login.wx2.qq.com/cgi-bin/mmwebwx-bin/login"
	InitURL            = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit"
	SyncCheckURL       = "https://webpush.wx2.qq.com/cgi-bin/mmwebwx-bin/synccheck"
	WebWXSyncURL       = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxsync"
	WebWXGetContactURL = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxgetcontact"
	WebWXSendMsgURL    = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxsendmsg"

	SelectorNewMessage = "2"
	SelectorNothing    = "0"
)

var (
	UUIDMatcher        = regexp.MustCompile("uuid\\s*=\\s*\"([^\"]*)\"\\s*")
	RedirectURLMatcher = regexp.MustCompile("redirect_uri\\s*=\"([^\"]*)\"")
	SelectorMatcher    = regexp.MustCompile("selector:\"(\\d+)\"")

	NilCredential WechatCredential

	CheckLoginTimeout = errors.New("Wechat CheckLogin Timeout")
	NoSuchClient      = errors.New("No such wechat client")
	SendMsgError      = errors.New("Send message error")
)

var botCenter = struct {
	sync.Mutex
	bots map[string]*WechatClient
}{
	bots: make(map[string]*WechatClient),
}

func NewWechatClient(tgBotName, accountId string) *WechatClient {
	botCenter.Lock()
	defer botCenter.Unlock()

	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})

	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			IdleConnTimeout: 3 * time.Second,
		},
		Timeout: 0,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	cli := &WechatClient{
		Client:   client,
		DeviceID: genDeviceID(),

		AccountId:   accountId,
		TelegramBot: tgBotName,
	}

	return cli
}

// GetByAccountId returns a current logged in
// Wechat account, else error
func GetByUserName(userName string) (*WechatClient, error) {
	botCenter.Lock()
	defer botCenter.Unlock()
	if bot, ok := botCenter.bots[userName]; ok {
		return bot, nil
	}
	return nil, NoSuchClient
}

// Login Steps
// 1. generate uuid
// 2. check login
// 3. init client
// 	3.1 get contact list
// 5. start sync
// 	5.1 get message
type WechatClient struct {
	Client      *http.Client
	NickName    string
	UserName    string
	DeviceID    string
	Credential  WechatCredential
	ContactList []*WechatFriend

	AccountId string
	// unique telegram bot name
	TelegramBot string
}

func (w *WechatClient) RegisterToCenter() {
	botCenter.Lock()
	defer botCenter.Unlock()
	botCenter.bots[w.UserName] = w
	logger.Println("new bot registered", w.UserName)
}

func (w *WechatClient) CheckLogin(uuid []byte) error {
	checkLoginURL := getCheckLoinURL(uuid)
	signals := make(chan int)
	quitSig := make(chan int)

	go func() {
		defer close(signals)
		for {
			select {
			case <-quitSig:
				logger.Println("Received quit signal, quitting")
				return
			default:
				logger.Println("Polling url", checkLoginURL.String())
				res, err := http.Get(checkLoginURL.String())
				if err != nil {
					logger.Printf("error check login %s\n", err)
					return
				}
				defer res.Body.Close()
				bs, err := ioutil.ReadAll(res.Body)
				if err != nil {
					logger.Printf("Error reading response: %s\n", err)
					return
				}
				if match, _ := regexp.Match("^window\\.code=201", bs); match {
					logger.Println("match 201")
					signals <- 201
				} else if match, _ := regexp.Match("^window\\.code=200", bs); match {
					logger.Println("match 200")
					// get the redirect uri
					redirectURI := string(getRedirectURI(bs))
					logger.Println("Found redirect_uri", redirectURI)
					// login
					res, err := w.Client.Get(redirectURI)
					if err != nil {
						logger.Printf("error fetching redirectURI %s\n", err)
						return
					}
					defer res.Body.Close()
					var logonRes LogonResponse
					if err := xml.NewDecoder(res.Body).Decode(&logonRes); err != nil {
						logger.Printf("error parsing logon response %s\n", err)
						return
					}
					logger.Printf("Got logon response, setting credentials...")
					w.setCredential(&logonRes)
					signals <- 200
					return
				}
			}
		}
	}()
L:
	for {
		select {
		case sig := <-signals:
			if sig == 201 {
				logger.Println("Avatar loaded...")
			} else if sig == 200 {
				logger.Println("Check Login succeeded")
				break L
			}
		case <-time.After(60 * time.Second):
			logger.Println("login timeout, sending quit signal")
			close(quitSig)
			return CheckLoginTimeout
		}
	}
	return nil
}

func (w *WechatClient) setCredential(logonRes *LogonResponse) {
	(&w.Credential).PassTicket = logonRes.PassTicket
	(&w.Credential).Sid = logonRes.Wxsid
	(&w.Credential).Skey = logonRes.Skey
	(&w.Credential).Uin = logonRes.Wxuin
}

func (w *WechatClient) SaveCredential() {
	var credential = models.WechatCredential{
		AccountId: w.AccountId,
	}

	u, _ := url.Parse(BaseCookieURL)
	var cookies = make(map[string]string)
	for _, cookie := range w.Client.Jar.Cookies(u) {
		cookies[cookie.Name] = cookie.Value
	}

	if _, err := models.Engine.Model(&credential).
		Where("account_id = ?account_id").
		Set("pass_ticket = ?", w.Credential.PassTicket).
		Set("sid = ?", w.Credential.Sid).
		Set("skey = ?", w.Credential.Skey).
		Set("username = ?", w.UserName).
		Set("telegram_bot = ?", w.TelegramBot).
		Set("status = ?", 1).
		Set("sync_key = ?", w.Credential.SyncKey.GetValue()).
		Set("cookies = ?", cookies).
		Set("updated = ?", time.Now().UTC()).SelectOrInsert(); err != nil {
		logger.Println("Error saving creadentials for wechat:", w.NickName, err)

	} else {
		credential.PassTicket = w.Credential.PassTicket
		credential.Sid = w.Credential.Sid
		credential.Skey = w.Credential.Skey
		credential.Username = w.UserName
		credential.TelegramBot = w.TelegramBot
		credential.Status = 1
		credential.SyncKey = w.Credential.SyncKey.GetValue()
		credential.Cookies = cookies
		credential.Updated = time.Now().UTC()
		if err := models.Engine.Update(&credential); err != nil {
			logger.Println("Error update existed credential for wechat: ", w.NickName, err)
		}
	}
}

func (w *WechatClient) updateSyncKey(syncKey *SyncKey) {
	var credential = new(models.WechatCredential)
	models.Engine.Model(&credential).
		Set("sync_key = ?", w.Credential.SyncKey.GetValue()).
		Where("sync_key = ?", syncKey.GetValue()).Update()
	(&w.Credential).SyncKey = *syncKey
}

func (w *WechatClient) InitClient() {
	u, _ := url.Parse(InitURL)
	q := u.Query()
	q.Set("r", getR())
	q.Set("pass_ticket", w.Credential.PassTicket)
	u.RawQuery = q.Encode()

	request := &InitRequest{
		BaseRequest: &BaseRequest{
			DeviceID: w.DeviceID,
			Sid:      w.Credential.Sid,
			Skey:     w.Credential.Skey,
			Uin:      w.Credential.Uin,
		},
	}
	var body = new(bytes.Buffer)
	json.NewEncoder(body).Encode(request)
	res, err := w.Client.Post(u.String(), "application/json", body)
	if err != nil {
		logger.Printf("Error init client %s\n", err)
		return
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var initRes InitResponse
	decoder.Decode(&initRes)
	w.NickName = initRes.User.NickName
	w.UserName = initRes.User.UserName
	(&w.Credential).SyncKey = initRes.SyncKey

	// get contact list
	w.getContactList()

}

func (w *WechatClient) getContactList() {
	u, _ := url.Parse(WebWXGetContactURL)
	q := u.Query()
	q.Set("lang", "en_US")
	q.Set("pass_ticket", w.Credential.PassTicket)
	q.Set("r", getR())
	q.Set("seq", "0")
	q.Set("skey", w.Credential.Skey)
	u.RawQuery = q.Encode()

	res, err := w.Client.Get(u.String())
	if err != nil {
		logger.Printf("Error get contact list")
		return
	}
	defer res.Body.Close()
	var response struct {
		BaseResponse BaseResponse
		MemberCount  int
		MemberList   []*WechatFriend
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err == nil {
		logger.Printf("Got %d contacts\n", response.MemberCount)
		w.ContactList = response.MemberList
	}
}

func (w *WechatClient) StartSyncCheck() {
	for {
		u, _ := url.Parse(SyncCheckURL)
		q := u.Query()
		q.Set("r", getR())
		q.Set("skey", w.Credential.Skey)
		q.Set("uin", w.Credential.Uin)
		q.Set("deviceid", w.DeviceID)
		q.Set("sid", w.Credential.Sid)

		syncKeys := make([]string, 0)
		for _, v := range w.Credential.SyncKey.List {
			syncKeys = append(syncKeys, fmt.Sprintf("%d_%d", v.Key, v.Val))
		}
		q.Set("synckey", strings.Join(syncKeys, "|"))
		q.Set("_", getT())
		u.RawQuery = q.Encode()

		logger.Println("Polling new msg for wechat client:", w.UserName)
		if res, err := w.Client.Get(u.String()); err != nil {
			logger.Printf("error syncing for account %s， err: %s\n", w.NickName, err)
		} else {
			bs, _ := ioutil.ReadAll(res.Body)
			selector := SelectorMatcher.FindStringSubmatch(string(bs))[1]
			switch selector {
			case SelectorNewMessage:
				logger.Println("Got wechat new message")
				w.getNewMessage()
			case SelectorNothing:
				continue
			default:
				logger.Println("Unexpected resonse, sleeping", string(bs))
				time.Sleep(5 * time.Second)
			}
			res.Body.Close()
		}
	}
}

func (w *WechatClient) SendMessage(msg SendMessage) error {
	logger.Println("Sending msg from ", msg.FromUserName, "to", msg.ToUserName, "content", msg.Content[:10])
	u, _ := url.Parse(WebWXSendMsgURL)
	q := u.Query()
	q.Set("lang", "en_US")
	q.Set("pass_ticket", w.Credential.PassTicket)
	u.RawQuery = q.Encode()

	id := strconv.FormatInt(time.Now().UnixNano(), 10)[:17]
	reqBody := SendMessageRequest{
		BaseRequest: w.getBaseRequest(),
		Msg: requestMessage{
			ClientMsgId:  id,
			Content:      msg.Content,
			FromUserName: msg.FromUserName,
			LocalID:      id,
			ToUserName:   msg.ToUserName,
			Type:         1,
		},
		Scene: 0,
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(reqBody)
	res, err := w.Client.Post(u.String(), "application/json", body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var sendResponse struct {
		BaseResponse BaseResponse
		MsgID        string
	}
	json.NewDecoder(res.Body).Decode(&sendResponse)
	if sendResponse.MsgID == "" {
		logger.Println("Error sending wx msg")
		return SendMsgError
	}
	return nil
}

func (w *WechatClient) getNewMessage() {
	u, _ := url.Parse(WebWXSyncURL)
	q := u.Query()
	q.Set("sid", w.Credential.Sid)
	q.Set("skey", w.Credential.Skey)
	q.Set("pass_ticket", w.Credential.PassTicket)
	q.Set("lang", "en_US")
	u.RawQuery = q.Encode()

	reqBody := struct {
		BaseRequest *BaseRequest
		SyncKey     SyncKey
		rr          string
	}{
		BaseRequest: w.getBaseRequest(),
		SyncKey:     w.Credential.SyncKey,
		rr:          getRR(),
	}

	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(reqBody)
	res, err := w.Client.Post(u.String(), "application/json", body)
	if err != nil {
		logger.Printf("error get new message %s\n", err)
		return
	}
	defer res.Body.Close()
	bs, _ := ioutil.ReadAll(res.Body)
	var syncRes WebwxSyncResponse
	json.Unmarshal(bs, &syncRes)
	w.updateSyncKey(&syncRes.SyncKey)

	for _, msg := range syncRes.AddMsgList {

		for _, user := range w.ContactList {
			if msg.FromUserName == user.UserName {

				logger.Printf("Got new msg from %s -> %s\n", msg.FromUserName, w.UserName)

				var saveMsg = models.Message{
					WechatMsgId:        msg.MsgId,
					WechatFromUser:     msg.FromUserName,
					WechatFromNickName: user.NickName,
					WechatToUser:       w.UserName,
					WechatToNickName:   w.NickName,
					Content:            msg.Content,
				}
				models.Engine.Model(&saveMsg).Insert()

				msgToTg := &queue.Message{
					FromType:  queue.TypeWechat,
					FromUser:  user.NickName,
					ToType:    queue.TypeTelegram,
					ToUser:    w.TelegramBot,
					Content:   msg.Content,
					FromMsgId: msg.MsgId,
				}
				queue.MessageSwitcher.Broadcast(msgToTg)
			}
		}
	}
}

func (w *WechatClient) getBaseRequest() *BaseRequest {
	return &BaseRequest{
		Uin:      w.Credential.Uin,
		Sid:      w.Credential.Sid,
		Skey:     w.Credential.Skey,
		DeviceID: w.DeviceID,
	}
}

func getR() string {
	return strconv.Itoa(rand.Int())
}

func getT() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func getRR() string {
	return strconv.FormatInt(-^time.Now().UnixNano(), 10)[:10]
}

func genDeviceID() string {
	return "e" + strconv.FormatFloat(rand.Float64(), 'f', 15, 64)[2:]
}

func getRedirectURI(raw []byte) []byte {
	return RedirectURLMatcher.FindSubmatch(raw)[1]
}

func getCheckLoinURL(uuid []byte) *url.URL {
	u, _ := url.Parse(CheckLoginURL)
	q := u.Query()
	q.Set("loginicon", "true")
	q.Set("uuid", string(uuid))
	q.Set("tip", "0")
	q.Set("r", getR())
	q.Set("_", getT())
	u.RawQuery = q.Encode()
	return u
}

func GetUUID() ([]byte, error) {
	req, err := http.Get(GetUUIDURL)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()
	txt, _ := ioutil.ReadAll(req.Body)
	return UUIDMatcher.FindSubmatch(txt)[1], nil
}
