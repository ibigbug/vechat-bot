package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"strconv"

	"io/ioutil"

	"log"

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

func (t *TelegramBot) String() string {
	return fmt.Sprintf("TelegramBot{Name: %s, ptr: %p}", t.Name, t)
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
	logger.Printf("GetMe %s\n", u.String())
	res, err := http.Get(u.String())
	if err == nil {
		defer res.Body.Close()
	}
	decoder := json.NewDecoder(res.Body)
	var response UserResponse
	err = decoder.Decode(&response)
	return response.Result, err
}

func (t *TelegramBot) SetDisable() {
	if _, err := models.Engine.Model(&models.TelegramBot{}).Where("name = ?", t.Name).Set("status = ?", 0).Update(); err == nil {
		logger.Println("Setting bot", t, "status to 0")
	} else {
		log.Println("Error setting bot", t, "to disable")
	}
}

func (t *TelegramBot) GetUpdates() {
	u, _ := url.Parse(TelegramAPIEndpoint)
	u.Path += fmt.Sprintf("/bot%s/getUpdates", t.Token)
	q := u.Query()
	q.Set("timeout", "30")
	u.RawQuery = q.Encode()

	for {
		req, _ := http.NewRequest("GET", u.String(), nil)
		logger.Printf("ping %s\n", t)
		if res, err := t.client.Do(req.WithContext(t.ctx)); err != nil {
			if uerr, ok := err.(*url.Error); ok {
				if uerr.Temporary() || uerr.Timeout() {
					logger.Printf("Error recoverable %s\n", uerr.Error())
					continue
				} else if uerr.Err == context.Canceled {
					logger.Printf("Update canceld... %s\n", t)
					break
				} else {
					logger.Printf("Error unrecoverable %s\n", uerr.Error())
					break
				}
			} else {
				logger.Printf("Unknown Error: %s\n", err.Error())
			}
		} else {
			defer res.Body.Close()
			var update UpdateResponse
			if err := json.NewDecoder(res.Body).Decode(&update); err != nil {
				logger.Println("Error decode tg message", err, t)
				continue
			}
			if !update.Ok {
				if update.ErrorCode == 409 {
					logger.Println("Error polling for bot", t, "error", update.Description, "Terminating...")
					t.SetDisable()
					break
				}
			}
			for _, up := range update.Result {
				logger.Println("Telegram bot", t, "got new message")
				if up.Message.Text == "/login" {
					logger.Printf("register with chat id %d\n", up.Message.Chat.Id)
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
						logger.Printf("Got reply to %s, content %s\n", record.WechatFromNickName, up.Message.Text)

						var msgToWx = &queue.Message{
							FromType:  queue.TypeTelegram,
							FromUser:  t.Name,
							FromMsgId: strconv.Itoa(up.Message.MessageId),
							ToType:    queue.TypeWechat,
							ToUser:    record.WechatToUser, // This message is send to which the wechat user friend was sen to
							Content:   up.Message.Text,
							Extra: map[string]string{
								"TargetFriend": record.WechatFromUser,
							},
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
		logger.Println("Error send message, need to retry", err, t)
		return nil, err
	}
	defer res.Body.Close()
	bs, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, errors.New(res.Status + ", error " + string(bs))
	}
	var rv SendMessageResponse
	json.Unmarshal(bs, &rv)
	return rv.Result, nil
}
