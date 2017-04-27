package wechat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
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
				log.Println("Received quit signal, quitting")
				return
			default:
				log.Println("Polling url", checkLoginURL.String())
				if res, err := http.Get(checkLoginURL.String()); err != nil {
					log.Printf("error check login %s\n", err)
					break
				} else {
					defer res.Body.Close()
					bs, err := ioutil.ReadAll(res.Body)
					if err != nil {
						log.Printf("Error reading response: %s\n", err)
						break
					}
					if match, _ := regexp.Match("^window\\.code=201", bs); match {
						signals <- 201
					} else if match, _ := regexp.Match("^window\\.code=200", bs); match {
						// get the redirect uri
						redirectURI := string(getRedirectURI(bs))
						log.Println("Found redirect_uri", redirectURI)
						// login
						if rv, err := w.Client.Get(redirectURI); err != nil {
							log.Printf("error fetching redirectURI %s\n", err)
							break
						} else {
							defer rv.Body.Close()
							rvbs, _ := ioutil.ReadAll(rv.Body)
							var logonRes LogonResponse
							if err := xml.Unmarshal(rvbs, &logonRes); err != nil {
								log.Printf("error parsing logon response %s,response: %s\n", err, string(rvbs))
								break
							}
							log.Printf("Got logon response, setting credentials...")
							w.setCredential(&logonRes)
							signals <- 200
							break
						}
					}
				}
			}
		}
	}()
L:
	for {
		select {
		case sig := <-signals:
			if sig == 201 {
				log.Println("Avatar loaded...")
				continue
			} else if sig == 200 {
				log.Println("Check Login succeeded")
				break L
			}
		case <-time.After(60 * time.Second):
			log.Println("login timeout, sending quit signal")
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
	var credential = new(models.WechatCredential)
	credential.PassTicket = w.Credential.PassTicket
	credential.Sid = w.Credential.Sid
	credential.Skey = w.Credential.Skey
	credential.Username = w.NickName
	credential.AccountId = w.AccountId
	credential.TelegramBot = w.TelegramBot
	credential.Status = 1

	var syncKey = make([]string, w.Credential.SyncKey.Count)
	for _, sk := range w.Credential.SyncKey.List {
		fmt.Println(sk.Key, sk.Val, "sync key")
		syncKey = append(syncKey, fmt.Sprintf("%s_%s", sk.Key, sk.Val))
	}
	credential.SyncKey = syncKey[:w.Credential.SyncKey.Count]

	u, _ := url.Parse(BaseCookieURL)
	var cookies = make(map[string]string)
	for _, cookie := range w.Client.Jar.Cookies(u) {
		cookies[cookie.Name] = cookie.Value
	}
	credential.Cookies = cookies
	if rv, err := models.Engine.Model(&credential).Insert(); err != nil {
		log.Println("Error saving credential:", err)
	} else {
		log.Println("credential saved for user", w.NickName, rv)
	}
}

func (w *WechatClient) updateSyncKey(syncKey *SyncKey) {
	(&w.Credential).SyncKey = *syncKey
	var credential = new(models.WechatCredential)
	models.Engine.Model(&credential).
		Set("sync_key = ?", syncKey).
		Where("sync_key = ?", syncKey).Update()
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
		log.Printf("Error init client %s\n", err)
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
		log.Printf("Error get contact list")
		return
	}
	defer res.Body.Close()
	var response struct {
		BaseResponse BaseResponse
		MemberCount  int
		MemberList   []*WechatFriend
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err == nil {
		log.Printf("Got %d contacts\n", response.MemberCount)
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

		log.Println("Synccheck with", u.String())
		if res, err := w.Client.Get(u.String()); err != nil {
			log.Printf("error syncing for account %s， err: %s\n", w.NickName, err)
		} else {
			bs, _ := ioutil.ReadAll(res.Body)
			selector := SelectorMatcher.FindStringSubmatch(string(bs))[1]
			switch selector {
			case SelectorNewMessage:
				log.Println("Got new message")
				w.getNewMessage()
			case SelectorNothing:
				continue
			default:
				log.Println("Unexpected resonse, sleeping", string(bs))
				time.Sleep(5 * time.Second)
			}
			res.Body.Close()
		}
	}
}

func (w *WechatClient) SendMessage(msg SendMessage) error {
	log.Println("Sending msg from ", msg.FromUserName, "to", msg.ToUserName, "content", msg.Content)
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
		log.Printf("error get new message %s\n", err)
		return
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var syncRes WebwxSyncResponse
	decoder.Decode(&syncRes)
	w.updateSyncKey(&syncRes.SyncKey)

	for _, user := range w.ContactList {
		for _, msg := range syncRes.AddMsgList {
			if msg.FromUserName == user.UserName {
				log.Printf("Got new msg from %s, content: %s\n", user.NickName, msg.Content)

				var saveMsg = models.Message{
					WechatMsgId:        msg.MsgId,
					WechatFromUser:     msg.FromUserName,
					WechatToUser:       msg.ToUserName,
					WechatFromNickName: user.NickName,
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
