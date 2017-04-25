package handlers

import (
	"fmt"
	"net/http"

	"strconv"

	"strings"

	"github.com/ibigbug/vechat-bot/middlewares"
	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/telegram"
)

func AddTelegramBotHandler(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middlewares.CtxKey("user"))
	if user == nil {
		http.Error(w, "LoginFirst", http.StatusUnauthorized)
		return
	}

	r.ParseForm()
	botName := strings.TrimSpace(r.PostForm.Get("bot_name"))
	botToken := strings.TrimSpace(r.PostForm.Get("bot_token"))
	if botName == "" || botToken == "" {
		http.Error(w, "Bot Name and Bot Token are required", http.StatusBadRequest)
		return
	}

	tgBot := models.TelegramBot{
		Name:      botName,
		Token:     botToken,
		AccountId: user.(models.GoogleAccount).Sub,
	}
	if err := models.Engine.Insert(&tgBot); err == nil {
		fmt.Println(tgBot)
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func ToggleBotStatusHandler(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middlewares.CtxKey("user"))
	if user == nil {
		http.Error(w, "LoginFirst", http.StatusUnauthorized)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing bot id", http.StatusBadRequest)
		return
	}
	Id, _ := strconv.Atoi(id)
	var bot = models.TelegramBot{
		Id: Id,
	}
	if err := models.Engine.Select(&bot); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if bot.Status == 0 {
		bot.Status = 1
		if _, err := models.Engine.Model(&bot).Column("status").Update(); err != nil {
			panic(err)
		}
		botClient := telegram.GetBotClient(bot.Token, bot.Name)
		botInfo, _ := botClient.GetMe()
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(botInfo.FirstName))
		go botClient.GetUpdates()
	} else {
		bot.Status = 0
		if _, err := models.Engine.Model(&bot).Column("status").Update(); err != nil {
			panic(err)
		}
		botClient := telegram.GetBotClient(bot.Token, bot.Name)
		botClient.CancelUpdate()
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("disabled"))
	}
}
