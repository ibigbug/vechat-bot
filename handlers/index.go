package handlers

import (
	"html/template"
	"net/http"

	"fmt"

	"github.com/ibigbug/vechat-bot/middlewares"
	"github.com/ibigbug/vechat-bot/models"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {

	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		panic(err)
	}
	user := r.Context().Value(middlewares.CtxKey("user"))

	locals := map[string]interface{}{
		"qrcode": "/qrcode",
		"user":   user,
	}

	if user != nil {
		var tgBots []models.TelegramBot
		models.Engine.Model(&tgBots).Where("account_id = ?", user.(models.GoogleAccount).Sub).Select()
		locals["tgbots"] = tgBots
	}
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, locals)
}

func WechatLoginPage(w http.ResponseWriter, r *http.Request) {
	if user := r.Context().Value(middlewares.CtxKey("user")); user != nil {
		var bot models.TelegramBot
		if err := models.Engine.Model(&bot).Where("id = ?", r.URL.Query().Get("bot")).Select(); err != nil {
			http.Error(w, "Sorry, not your bot", http.StatusForbidden)
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<p>Scan QRCode using Wechat</p>`))
			w.Write([]byte("<small>Refresh if QRCode failed to load</small>"))
			w.Write([]byte(fmt.Sprintf(`<img src="/qrcode?bot=%s" />`, bot.Name)))
		}
	} else {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

}
