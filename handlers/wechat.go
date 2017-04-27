package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ibigbug/vechat-bot/middlewares"
	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/wechat"
	qrcode "github.com/skip2/go-qrcode"
)

func QRCodeHandler(w http.ResponseWriter, r *http.Request) {
	if user := r.Context().Value(middlewares.CtxKey("user")); user != nil {
		var bot models.TelegramBot
		if err := models.Engine.Model(&bot).
			Where("account_id = ?", user.(models.GoogleAccount).Sub).
			Where("name = ?", r.URL.Query().Get("bot")).Select(); err != nil {
			http.Error(w, "Not your bot", http.StatusForbidden)
		} else {
			uuid, err := wechat.GetUUID()
			if err != nil {
				http.Error(w, "Failed to get UUID", http.StatusSeeOther)
				return
			}
			png, err := qrcode.Encode(fmt.Sprintf("https://login.weixin.qq.com/l/%s", uuid), qrcode.Highest, 256)
			if err != nil {
				panic(err)
			}
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "image/png")
			w.Write(png)

			go func() {
				if oldClient, err := wechat.GetByTelegramBot(bot.Name); err == nil {
					log.Println("cancelling old wechat client for tgBot", bot.Name)
					oldClient.CancelSynCheck()
					oldClient.Destroy()
				}
				wxClient := wechat.NewWechatClient(bot.Name, user.(models.GoogleAccount).Sub)
				if err := wxClient.CheckLogin(uuid); err != nil {
					if err == wechat.CheckLoginTimeout {
						log.Println("CheckLogin timeout, goodbye")
					} else {
						log.Println("Error occured", err)
					}
					return
				}
				wxClient.InitClient()
				wxClient.SaveCredential()
				wxClient.RegisterToCenter()
				wxClient.StartSyncCheck()
				log.Println("Polling exited.. sth wrong might happend...")
			}()
		}
	} else {
		http.Error(w, "Need login", http.StatusUnauthorized)
	}
}
