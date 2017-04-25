package handlers

import (
	"html/template"
	"net/http"

	"fmt"

	"github.com/ibigbug/vechat-bot/middlewares"
	"github.com/ibigbug/vechat-bot/models"
	"github.com/ibigbug/vechat-bot/wechat"
	qrcode "github.com/skip2/go-qrcode"
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
		var channels []models.ChannelBinding
		models.Engine.Model(&channels).Where("account_id = ?", user.(models.GoogleAccount).Sub).Select()
		locals["bindings"] = channels

		var tgBots []models.TelegramBot
		models.Engine.Model(&tgBots).Where("account_id = ?", user.(models.GoogleAccount).Sub).Select()
		locals["tgbots"] = tgBots
	}
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, locals)

}

func QRCodeHandler(w http.ResponseWriter, r *http.Request) {
	uuid, err := wechat.GetUUID()
	png, err := qrcode.Encode(fmt.Sprintf("https://login.weixin.qq.com/l/%s", uuid), qrcode.Medium, 256)
	if err != nil {
		panic(err)
	}
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)

	go func() {
		wxClient := &wechat.WechatClient{}
		wxClient.CheckLogin(uuid)

		fmt.Println("Still polling.. sth wrong might happend...")

	}()
}
