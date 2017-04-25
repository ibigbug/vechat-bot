package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"strconv"

	"github.com/ibigbug/vechat-bot/models"
)

const (
	TelegramAPIEndpoint = "https://api.telegram.org"
)

func init() {
	log.Println("Surviving bots")
	var bots []models.TelegramBot
	if err := models.Engine.Model(&bots).Where("status = ?", 1).Select(); err == nil {
		log.Printf("Got %d bots to survive\n", len(bots))
		for _, b := range bots {
			cli := GetBotClient(b.Token, b.Name)
			go cli.GetUpdates()
			log.Printf("Surived a bot %s\n", b.Name)
		}
	} else {
		log.Printf("error occured while surviving bots: %s\n", err.Error())
	}
}

var botCenter = struct {
	sync.Mutex
	bots map[string]*TelegramBot
}{
	bots: make(map[string]*TelegramBot),
}

func GetBotClient(token, name string) *TelegramBot {
	botCenter.Lock()
	defer botCenter.Unlock()
	if bot, ok := botCenter.bots[token]; ok {
		return bot
	}

	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		IdleConnTimeout:     3 * time.Second,
		TLSHandshakeTimeout: 3 * time.Second,
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	bot := &TelegramBot{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		client: http.Client{
			Transport: tr,
		},
		Token: token,
		Name:  name,
	}
	botCenter.bots[token] = bot
	return bot
}

type TelegramBot struct {
	sync.Mutex
	ctx        context.Context
	cancelFunc context.CancelFunc
	client     http.Client
	Token      string
	Name       string
	ChatId     int64
}

func (t *TelegramBot) CancelUpdate() {
	t.Lock()
	defer t.Unlock()
	t.cancelFunc()
	ctx, cancel := context.WithCancel(context.Background())
	t.ctx = ctx
	t.cancelFunc = cancel
}

func (t *TelegramBot) GetMe() (user User, err error) {
	u, _ := url.Parse(TelegramAPIEndpoint)
	u.Path += fmt.Sprintf("/bot%s/getMe", t.Token)
	log.Printf("GetMe %s\n", u.String())
	res, err := http.Get(u.String())
	if err == nil {
		defer res.Body.Close()
	}
	decoder := json.NewDecoder(res.Body)
	var response UserResponse
	err = decoder.Decode(&response)
	return response.Result, err
}

func (t *TelegramBot) GetUpdates() {
	u, _ := url.Parse(TelegramAPIEndpoint)
	u.Path += fmt.Sprintf("/bot%s/getUpdates", t.Token)
	q := u.Query()
	q.Set("timeout", "10")
	u.RawQuery = q.Encode()

	for {
		req, _ := http.NewRequest("GET", u.String(), nil)
		log.Printf("Geting updates %s\n", req.URL.String())
		if rv, err := t.client.Do(req.WithContext(t.ctx)); err != nil {
			if uerr, ok := err.(*url.Error); ok {
				if uerr.Temporary() || uerr.Timeout() {
					log.Printf("Error recoverable %s\n", uerr.Error())
					continue
				} else if uerr.Err == context.Canceled {
					log.Printf("Update canceld... %s\n", t.Name)
					break
				} else {
					log.Printf("Error unrecoverable %s\n", uerr.Error())
					break
				}
			} else {
				log.Printf("Unknown Error: %s\n", err.Error())
			}
		} else {
			decoder := json.NewDecoder(rv.Body)
			var update UpdateResponse
			decoder.Decode(&update)
			log.Printf("Got %d messages\n", len(update.Result))
			for _, up := range update.Result {
				log.Printf("Msg %s, %d\n", up.Message, up.UpdateId)
				if up.Message.Text == "/login" {
					log.Printf("register with chat id %d\n", up.Message.Chat.Id)
					t.ChatId = up.Message.Chat.Id
				}

				q.Set("offset", strconv.FormatInt(up.UpdateId+1, 10))
				u.RawQuery = q.Encode()
			}
		}
	}
}

func (t *TelegramBot) SendMessage(msg SendMessage) {
	msg.ChatId = t.ChatId
	u, _ := url.Parse(TelegramAPIEndpoint)
	u.Path += fmt.Sprintf("/bot%s/sendMessage", t.Token)
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(msg)
	res, err := t.client.Post(u.String(), "application/json", body)
	if err != nil {
		log.Println("Error send message, need to retry")
		return
	}
	log.Println("Send msg to bot", res.Status)
}
