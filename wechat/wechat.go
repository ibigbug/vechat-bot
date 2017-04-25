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
	"time"

	"golang.org/x/net/publicsuffix"
)

const (
	BaseCookieURL = "https://wxq.qq.com"
	GetUUIDURL    = "https://login.wx2.qq.com/jslogin?appid=wx782c26e4c19acffb&redirect_uri=https%3A%2F%2Fwx.qq.com%2Fcgi-bin%2Fmmwebwx-bin%2Fwebwxnewloginpage&fun=new&lang=en_US&_=1492959953169"
	CheckLoginURL = "https://login.wx2.qq.com/cgi-bin/mmwebwx-bin/login"
	InitURL       = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit"
	SyncCheckURL  = "https://webpush.wx2.qq.com/cgi-bin/mmwebwx-bin/synccheck"
	WebWXSyncURL  = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxsync"
)

var UUIDMatcher = regexp.MustCompile("uuid\\s*=\\s*\"([^\"]*)\"\\s*")
var RedirectURLMatcher = regexp.MustCompile("redirect_uri\\s*=\"([^\"]*)\"")
var SelectorMatcher = regexp.MustCompile("selector:\"2\"")

func NewWechatClient(username, deviceId string, cookies []*http.Cookie, credential WechatCredential) *WechatClient {
	jar, _ := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	u, _ := url.Parse(BaseCookieURL)
	jar.SetCookies(u, cookies)

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
	if deviceId == "" {
		deviceId = genDeviceID()
	}
	return &WechatClient{
		Client:     client,
		Username:   username,
		DeviceID:   deviceId,
		Credential: credential,
	}
}

type WechatClient struct {
	Client     *http.Client
	Username   string
	DeviceID   string
	Credential WechatCredential
}

func getR() string {
	return strconv.Itoa(rand.Int())
}

func getT() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func genDeviceID() string {
	return strconv.FormatFloat(rand.Float64(), 'f', 15, 64)[2:]
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

func (w WechatClient) CheckLogin(uuid []byte) {
	checkLoginURL := getCheckLoinURL(uuid)
	fmt.Println("Polling url", checkLoginURL.String())
	signals := make(chan int)
	select {
	case <-signals:
		w.InitClient()
	case <-time.NewTicker(10 * time.Second).C:
		log.Println("login timeout")
	}

	go func() {
		for {
			if res, err := http.Get(checkLoginURL.String()); err != nil {
				log.Printf("error check login %s\n", err)
				time.Sleep(5)
				continue
			} else {
				defer res.Body.Close()
				bs, err := ioutil.ReadAll(res.Body)
				if match, _ := regexp.Match("^window\\.code=200", bs); match {
					// get the redirect uri
					redirectURI := string(getRedirectURI(bs))
					log.Println("Found redirect_uri", redirectURI)
					// login
					if rv, err := w.Client.Get(redirectURI); err != nil {
						log.Printf("error fetching redirectURI %s\n", err)
						time.Sleep(5)
						continue
					} else {
						defer rv.Body.Close()
						rvbs, _ := ioutil.ReadAll(rv.Body)
						var logonRes LogonResponse
						if err := xml.Unmarshal(rvbs, &logonRes); err != nil {
							log.Printf("error parsing logon response %s,response: %s\n", err, string(rvbs))
							time.Sleep(5)
							continue
						}
						fmt.Printf("%v\n", logonRes)
						w.setCredential(&logonRes)
						signals <- 1
						break
					}
				}
			}
		}
	}()
}

func (w WechatClient) setCredential(logonRes *LogonResponse) {
	(&w.Credential).PassTicket = logonRes.PassTicket
	(&w.Credential).Sid = logonRes.Wxsid
	(&w.Credential).Skey = logonRes.Skey
	(&w.Credential).Uin = logonRes.Wxuin
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

func (w WechatClient) InitClient() {
	u, _ := url.Parse(InitURL)
	q := u.Query()
	q.Set("r", getR())
	q.Set("pass_ticket", w.Credential.PassTicket)
	u.RawQuery = q.Encode()

	request := &Request{
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

	w.Username = initRes.User.UserName
	(&w.Credential).SyncKey = initRes.SyncKey
}

func (w WechatClient) StartSyncCheck() {

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

	for {
		fmt.Println("Synccheck with", u.String())
		if res, err := w.Client.Get(u.String()); err != nil {
			log.Printf("error syncing for account %s， err: %s\n", w.Username, err)
		} else {
			defer res.Body.Close()
			fmt.Println("Polling synccheck")
			bs, _ := ioutil.ReadAll(res.Body)
			selector2 := SelectorMatcher.Match(bs)
			if selector2 {
				w.GetNewMessage()
			}
		}
	}
}

func (w WechatClient) GetNewMessage() {
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
		rr          int
	}{
		BaseRequest: &BaseRequest{
			Uin:      w.Credential.Uin,
			Sid:      w.Credential.Sid,
			Skey:     w.Credential.Skey,
			DeviceID: w.DeviceID,
		},
		SyncKey: w.Credential.SyncKey,
		rr:      196602270,
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
	fmt.Printf("%v\n", syncRes)
	initRes.SyncKey = syncRes.SyncKey
	(&w.Credential).SyncKey = syncRes.SyncKey

	for _, user := range initRes.ContactList {
		for _, msg := range syncRes.AddMsgList {
			if msg.FromUserName == user.UserName {
				fmt.Printf("Got new msg from %s, content: %s\n", user.DisplayName, msg.Content)
			}
		}
	}
}
