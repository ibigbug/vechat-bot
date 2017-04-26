package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"strconv"

	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/queue"
)

const (
	TelegramAPIEndpoint = "https://api.telegram.org"
)

var (
	NoSuchTelegramBot = errors.New("No such Telegram bot")
)

var botCenter = struct {
	sync.Mutex
	bots map[string]*TelegramBot
}{
	bots: make(map[string]*TelegramBot),
}

// GetBotClient is global Telegram bot center
func GetBotClient(token, name string) *TelegramBot {
	botCenter.Lock()
	defer botCenter.Unlock()
	if bot, ok := botCenter.bots[name]; ok {
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
	botCenter.bots[name] = bot
	return bot
}

func GetBotByName(name string) (*TelegramBot, error) {
	botCenter.Lock()
	defer botCenter.Unlock()
	if bot, ok := botCenter.bots[name]; ok {
		return bot, nil
	}
	return nil, NoSuchTelegramBot
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

				if up.Message.Text == "/login" {
					log.Printf("register with chat id %d\n", up.Message.Chat.Id)
					t.ChatId = up.Message.Chat.Id
					var bot models.TelegramBot
					models.Engine.Model(&bot).
						Where("name = ?", t.Name).
						Column("updated").
						Set("chat_id = ?", t.ChatId).Update()
				} else {
					if replyMsg := up.Message.ReplyToMessage; replyMsg != nil {
						var record models.Message
						models.Engine.Model(&record).
							Where("telegram_chat_id = ?", t.ChatId).
							Where("telegram_msg_id = ?", replyMsg.MessageId).Select()
						log.Printf("Got reply to %s, content %s\n", record.WechatFromNickName, up.Message.Text)

						var msgToWx = &queue.Message{
							FromType:  queue.TypeWechat,
							FromUser:  t.Name,
							FromMsgId: strconv.Itoa(up.Message.MessageId),
							ToType:    queue.TypeWechat,
							ToUser:    record.WechatToUser, // This message is send to which the wechat user friend was sen to
							Extra: map[string]string{
								"TargetFriend": record.WechatFromUser,
							},
							Content: up.Message.Text,
						}
						queue.MessageSwitcher.Broadcast(msgToWx)
					}
				}

				q.Set("offset", strconv.FormatInt(up.UpdateId+1, 10))
				u.RawQuery = q.Encode()
			}
		}
	}
}

func (t *TelegramBot) SendMessage(msg SendMessage) (*Message, error) {
	msg.ChatId = t.ChatId
	u, _ := url.Parse(TelegramAPIEndpoint)
	u.Path += fmt.Sprintf("/bot%s/sendMessage", t.Token)
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(msg)
	res, err := t.client.Post(u.String(), "application/json", body)
	if err != nil {
		log.Println("Error send message, need to retry")
		return nil, err
	}
	defer res.Body.Close()
	log.Println("Send msg to bot", res.Status)
	var rv SendMessageResponse
	if err := json.NewDecoder(res.Body).Decode(&rv); err != nil {
		return nil, err
	}
	return rv.Result, nil
}
