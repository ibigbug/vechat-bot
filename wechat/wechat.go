package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	raven "github.com/getsentry/raven-go"
	"github.com/go-pg/pg"
	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
	"golang.org/x/net/publicsuffix"
)

const (
	BaseCookieURL      = "https://wx2.qq.com"
	GetUUIDURL         = "https://login.wx2.qq.com/jslogin?appid=wx782c26e4c19acffb&redirect_uri=https%3A%2F%2Fwx.qq.com%2Fcgi-bin%2Fmmwebwx-bin%2Fwebwxnewloginpage&fun=new&lang=en_US&_=1492959953169"
	CheckLoginURL      = "https://login.wx2.qq.com/cgi-bin/mmwebwx-bin/login"
	InitURL            = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit"
	SyncCheckURL       = "https://webpush.wx2.qq.com/cgi-bin/mmwebwx-bin/synccheck"
	WebWXSyncURL       = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxsync"
	WebWXGetContactURL = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxgetcontact"
	WebWXSendMsgURL    = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxsendmsg"

	UserAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36"
	ApplicationJson = "application/json"

	RetCodeOK          = "0"
	SelectorNewMessage = "2"
	SelectorNothing    = "0"
)

const (
	maxRetryTimes = 10
)

var (
	UUIDMatcher        = regexp.MustCompile("uuid\\s*=\\s*\"([^\"]*)\"\\s*")
	RedirectURLMatcher = regexp.MustCompile("redirect_uri\\s*=\"([^\"]*)\"")
	RetcodeMatcher     = regexp.MustCompile("retcode:\"(\\d+)\"")
	SelectorMatcher    = regexp.MustCompile("selector:\"(\\d+)\"")

	CheckLoginTimeout = errors.New("Wechat CheckLogin Timeout")
	NoSuchClient      = errors.New("No such wechat client")
	SendMsgError      = errors.New("Send message error")
	UpdateColumns     = []string{
		"cookies", "pass_ticket", "sid", "skey", "uin", "device_id", "username", "status", "sync_key", "updated",
	}
)

var botCenter = struct {
	sync.Mutex
	bots map[string]*WechatClient
}{
	bots: make(map[string]*WechatClient),
}

func New(tgBotName, accountId string) *WechatClient {
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

	ctx, cancel := context.WithCancel(context.Background())

	cli := &WechatClient{
		Client:   client,
		DeviceID: genDeviceID(),

		AccountId:   accountId,
		TelegramBot: tgBotName,

		ctx:        ctx,
		cancelFunc: cancel,

		retryTimes: 0,
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

func GetByTelegramBot(tgBot string) (*WechatClient, error) {
	botCenter.Lock()
	defer botCenter.Unlock()
	for u, bot := range botCenter.bots {
		logger.Println(u, bot)
		if bot.TelegramBot != "" && bot.TelegramBot == tgBot {
			return bot, nil
		}
	}
	return nil, NoSuchClient
}

func FromCredential(credential *models.WechatCredential) *WechatClient {
	var bot = New(credential.TelegramBot, credential.AccountId)
	bot.DeviceID = credential.DeviceID
	bot.Credential = Credential{
		PassTicket: credential.PassTicket,
		Sid:        credential.Sid,
		Skey:       credential.Skey,
		Uin:        credential.Uin,
		SyncKey:    buildSyncKey(credential.SyncKey),
	}

	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	var cookies = make([]*http.Cookie, 0, len(credential.Cookies))
	var u, _ = url.Parse(BaseCookieURL)
	for key, val := range credential.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:   key,
			Value:  val,
			Path:   "/",
			Domain: u.Hostname(),
		})
	}
	jar.SetCookies(u, cookies)
	bot.Client.Jar = jar
	return bot
}

// Login Steps
// 1. generate uuid
// 2. check login
// 3. init client
// 	3.1 get contact list
// 5. start sync
// 	5.1 get message
type WechatClient struct {
	sync.Mutex
	Client      *http.Client
	NickName    string
	UserName    string
	DeviceID    string
	Credential  Credential
	ContactList []*Friend

	AccountId string
	// unique telegram bot name
	TelegramBot string

	ctx        context.Context
	cancelFunc context.CancelFunc

	retryTimes int
}

func (w *WechatClient) String() string {
	return fmt.Sprintf("WechatClient{NickName: %s, ptr: %p, TelegramBot: %s}", w.NickName, w, w.TelegramBot)
}

func (w *WechatClient) RegisterToCenter() {
	botCenter.Lock()
	defer botCenter.Unlock()
	botCenter.bots[w.UserName] = w
	logger.Println("new bot registered", w.UserName)
}

func (w *WechatClient) Destroy() {
	botCenter.Lock()
	defer botCenter.Unlock()
	delete(botCenter.bots, w.UserName)
	logger.Println("wechat bot", w.UserName, "destroyed")
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
					// login & pour cookiejar
					req, err := http.NewRequest("GET", redirectURI, nil)
					req.Header.Set("User-Agent", UserAgent)
					res, err := w.Client.Do(req)
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
		case <-time.After(100 * time.Second):
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
	var exist = models.WechatCredential{}
	var credential = models.WechatCredential{
		AccountId: w.AccountId,
	}

	u, _ := url.Parse(BaseCookieURL)
	var cookies = make(map[string]string)
	for _, cookie := range w.Client.Jar.Cookies(u) {
		cookies[cookie.Name] = cookie.Value
	}
	credential.Cookies = cookies
	credential.PassTicket = w.Credential.PassTicket
	credential.Sid = w.Credential.Sid
	credential.Skey = w.Credential.Skey
	credential.Uin = w.Credential.Uin
	credential.DeviceID = w.DeviceID
	credential.Username = w.UserName
	credential.TelegramBot = w.TelegramBot
	credential.Status = 1
	credential.SyncKey = w.Credential.SyncKey.GetValue()
	credential.Updated = time.Now().UTC()

	if err := models.Engine.Model(&exist).Where("account_id = ?", credential.AccountId).Select(); err == pg.ErrNoRows {
		if rv, err := models.Engine.Model(&credential).Insert(); err != nil {
			logger.Println("Error update/insert credential for wechat: ", w.NickName, err, rv)
		} else {
			logger.Println("credential inserted", w)
		}
	} else if err == nil {
		if _, err := models.Engine.Model(&credential).Where("account_id = ?", w.AccountId).Column(UpdateColumns...).Update(); err == nil {
			logger.Println("credential updated", w)
		} else {
			logger.Println("Error updating credential", w, err)
		}
	} else {
		log.Println("Save credential failed", err, w)
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
	req, err := http.NewRequest("POST", u.String(), body)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", ApplicationJson)
	res, err := w.Client.Do(req)
	if err != nil {
		logger.Printf("Error init client %s\n", err)
		return
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var initRes InitResponse
	decoder.Decode(&initRes)

	// TODO check init success
	w.NickName = initRes.User.NickName
	w.UserName = initRes.User.UserName
	(&w.Credential).SyncKey = initRes.SyncKey

	// get contact list
	w.getContactList()

}

func (w *WechatClient) getContactList() {
	logger.Println("Getting cantacts for", w)
	u, _ := url.Parse(WebWXGetContactURL)
	q := u.Query()
	q.Set("lang", "en_US")
	q.Set("pass_ticket", w.Credential.PassTicket)
	q.Set("r", getR())
	q.Set("seq", "0")
	q.Set("skey", w.Credential.Skey)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("User-Agent", UserAgent)
	res, err := w.Client.Do(req)
	if err != nil {
		logger.Printf("Error get contact list")
		return
	}
	defer res.Body.Close()
	var response struct {
		BaseResponse BaseResponse
		MemberCount  int
		MemberList   []*Friend
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err == nil {
		logger.Printf("Got %d contacts\n", response.MemberCount)
		w.ContactList = response.MemberList
	}
}

func (w *WechatClient) CancelSynCheck() {
	logger.Println("cancelling bot", w)
	w.Lock()
	defer w.Unlock()
	w.cancelFunc()
	ctx, cancel := context.WithCancel(context.Background())
	w.ctx = ctx
	w.cancelFunc = cancel
}

func (w *WechatClient) StartSyncCheck() {
	logger.Println("ping", w)

	u, _ := url.Parse(SyncCheckURL)
	q := u.Query()
	q.Set("r", getR())
	q.Set("skey", w.Credential.Skey)
	q.Set("uin", w.Credential.Uin)
	q.Set("deviceid", w.DeviceID)
	q.Set("sid", w.Credential.Sid)
	q.Set("_", getT())

	for {
		q.Set("synckey", strings.Join(w.Credential.SyncKey.GetValue(), "|"))
		u.RawQuery = q.Encode()
		req, _ := http.NewRequest("GET", u.String(), nil)
		req.Header.Set("User-Agent", UserAgent)
		res, err := w.Client.Do(req.WithContext(w.ctx))
		if err != nil {
			if uerr, ok := err.(*url.Error); ok {
				logger.Println(uerr)
				if uerr.Temporary() || uerr.Timeout() {
					logger.Printf("Error recoverable %s\n", uerr.Error())
					time.Sleep(5 * time.Second)
					continue
				} else if uerr.Err == context.Canceled {
					logger.Printf("Update canceld... %s\n", w)
					w.Destroy()
					return
				} else {
					if w.retryTimes >= maxRetryTimes {
						logger.Printf("Error unrecoverable and maxRetryTimes exceeded %s\n", uerr.Error())
						raven.CaptureError(uerr, map[string]string{
							"NickName":  w.NickName,
							"AccountID": w.AccountId,
						})
						w.Destroy()
						return
					} else {
						w.retryTimes += 1
						time.Sleep(5 * time.Second)
						logger.Printf("Error occured, retrying, retry times: %d, err: %s", w.retryTimes, uerr.Error())
						continue
					}
				}
			} else {
				logger.Printf("error syncing for account %s， err: %s\n", w, err)
				return
			}
		}

		bs, _ := ioutil.ReadAll(res.Body)
		logger.Println("Syncecheck result", string(bs), w)
		selector, err := getSelector(bs)
		if err != nil {
			logger.Println("Error get synccheck result", err, w)
			time.Sleep(10)
			continue
		}
		if selector.retcode != "0" {
			logger.Println("Synccheck over, exit now")
			return
		}
		switch selector.selector {
		case SelectorNewMessage:
			logger.Println("Got wechat new message", w)
			w.getNewMessage()
		case SelectorNothing:
			logger.Println("Found nothing, still polling", w)
			res.Body.Close()
			continue
		default:
			logger.Println("Unexpected response, sleeping", w)
			time.Sleep(5 * time.Second)
		}
		res.Body.Close()

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
	req, _ := http.NewRequest("POST", u.String(), body)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", ApplicationJson)
	res, err := w.Client.Do(req)
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
	req, _ := http.NewRequest("POST", u.String(), body)
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", ApplicationJson)
	res, err := w.Client.Do(req)
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
		if msg.FromUserName == w.UserName {
			continue
		}
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

func getSelector(s []byte) (rv struct{ retcode, selector string }, err error) {
	m1 := RetcodeMatcher.FindSubmatch(s)
	m2 := SelectorMatcher.FindSubmatch(s)
	if len(m1) != 2 || len(m2) != 2 {
		return rv, errors.New("Invalid SyncCheckREsult")
	}
	rv.retcode = string(m1[1])
	rv.selector = string(m2[1])
	return
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

func buildSyncKey(sk []string) SyncKey {
	var rv = SyncKey{
		Count: len(sk),
		List:  make([]struct{ Key, Val int }, len(sk)),
	}
	for i, k := range sk {
		parts := strings.Split(k, "_")
		key, _ := strconv.Atoi(parts[0])
		val, _ := strconv.Atoi(parts[1])
		rv.List[i] = struct {
			Key, Val int
		}{
			Key: key,
			Val: val,
		}
	}
	return rv
}
