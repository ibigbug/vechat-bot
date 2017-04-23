package vechatsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DEVICE_ID       = "one-device"
	GET_UUID_URL    = "https://login.wx2.qq.com/jslogin?appid=wx782c26e4c19acffb&redirect_uri=https%3A%2F%2Fwx.qq.com%2Fcgi-bin%2Fmmwebwx-bin%2Fwebwxnewloginpage&fun=new&lang=en_US&_=1492959953169"
	CHECK_LOGIN_URL = "https://login.wx2.qq.com/cgi-bin/mmwebwx-bin/login"
	INIT_URL        = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxinit"
	SYNC_CHECK_URL  = "https://webpush.wx2.qq.com/cgi-bin/mmwebwx-bin/synccheck"
	WEBWX_SYNC_URL  = "https://wx2.qq.com/cgi-bin/mmwebwx-bin/webwxsync"
)

var UUID_MATCHER = regexp.MustCompile("uuid\\s*=\\s*\"([^\"]*)\"\\s*")
var REDIRECT_URL_MATCHER = regexp.MustCompile("redirect_uri\\s*=\"([^\"]*)\"")
var SELECTOR_MATHER = regexp.MustCompile("selector:\"2\"")

func GetUUID() []byte {
	req, _ := http.Get(GET_UUID_URL)
	defer req.Body.Close()
	txt, _ := ioutil.ReadAll(req.Body)
	return UUID_MATCHER.FindAllSubmatch(txt, -1)[0][1]
}

func GetCheckLoinURL(uuid []byte) *url.URL {
	// TODO: url parameter encode
	rand.Seed(99)
	r := rand.Int()
	t := time.Now().Unix()
	u, _ := url.Parse(CHECK_LOGIN_URL)
	q := u.Query()
	q.Set("loginicon", "true")
	q.Set("uuid", string(uuid))
	q.Set("tip", "0")
	q.Set("r", strconv.Itoa(r))
	q.Set("_", strconv.Itoa(int(t)))
	u.RawQuery = q.Encode()
	return u
}

func GetRedirectURL(raw []byte) []byte {
	return REDIRECT_URL_MATCHER.FindAllSubmatch(raw, -1)[0][1]
}

func InitClient(c *http.Client, logonResponse *LogonResponse) *InitResponse {
	rand.Seed(100)
	r := rand.Int()
	u, _ := url.Parse(INIT_URL)
	q := u.Query()
	q.Set("r", strconv.Itoa(r))
	q.Set("pass_ticket", logonResponse.PassTicket)
	u.RawQuery = q.Encode()

	br := &BaseRequest{
		DeviceID: DEVICE_ID,
		Sid:      logonResponse.Wxsid,
		Skey:     logonResponse.Skey,
		Uin:      logonResponse.Wxuin,
	}

	request := &Request{
		BaseRequest: br,
	}

	jsonValue, err := json.Marshal(request)
	if err != nil {
		fmt.Println(err)
	}

	req, _ := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonValue))

	res, err := c.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var initRes InitResponse
	decoder.Decode(&initRes)
	return &initRes
}

func StartSyncCheck(c *http.Client, initRes *InitResponse, logonResponse *LogonResponse) {
	rand.Seed(87)
	r := rand.Int()
	t := time.Now().Unix()

	u, _ := url.Parse(SYNC_CHECK_URL)
	q := u.Query()
	q.Set("r", strconv.Itoa(r))
	q.Set("skey", initRes.Skey)
	q.Set("uin", strconv.Itoa(initRes.User.Uin))
	q.Set("deviceid", DEVICE_ID)
	q.Set("sid", logonResponse.Wxsid)

	syncKeys := make([]string, 0)
	for _, v := range initRes.SyncKey.List {
		syncKeys = append(syncKeys, fmt.Sprintf("%d_%d", v.Key, v.Val))
	}
	q.Set("synckey", strings.Join(syncKeys, "|"))
	q.Set("_", strconv.Itoa(int(t)))

	u.RawQuery = q.Encode()

	for {
		fmt.Println("Synccheck with", u.String())
		res, _ := c.Get(u.String())
		defer res.Body.Close()
		fmt.Println("Polling synccheck")
		bs, _ := ioutil.ReadAll(res.Body)
		selector_2 := SELECTOR_MATHER.Match(bs)
		if selector_2 {
			go GetNewMessage(c, initRes, logonResponse)
		}
	}
}

func GetNewMessage(c *http.Client, initRes *InitResponse, logonRes *LogonResponse) {
	u, _ := url.Parse(WEBWX_SYNC_URL)
	q := u.Query()
	q.Set("sid", logonRes.Wxsid)
	q.Set("skey", logonRes.Skey)
	q.Set("pass_ticket", logonRes.PassTicket)
	q.Set("lang", "en_US")
	u.RawQuery = q.Encode()

	reqBody := struct {
		BaseRequest *BaseRequest
		SyncKey     struct {
			Count int
			List  []struct {
				Key int
				Val int
			}
		}
		rr int
	}{
		BaseRequest: &BaseRequest{
			Uin:      logonRes.Wxuin,
			Sid:      logonRes.Wxsid,
			Skey:     logonRes.Skey,
			DeviceID: DEVICE_ID,
		},
		SyncKey: initRes.SyncKey,
		rr:      196602270,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POSST", u.String(), bytes.NewBuffer(jsonBody))
	res, _ := c.Do(req)
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var syncRes WebwxSyncResponse
	decoder.Decode(&syncRes)
	fmt.Printf("%v\n", syncRes)
	initRes.SyncKey = syncRes.SyncKey

	for _, user := range initRes.ContactList {
		for _, msg := range syncRes.AddMsgList {
			if msg.FromUserName == user.UserName {
				fmt.Printf("Got new msg from %s, content: %s\n", user.DisplayName, msg.Content)
			}
		}
	}
}
