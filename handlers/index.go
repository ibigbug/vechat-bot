package handlers

import (
	"html/template"
	"net/http"

	"golang.org/x/net/publicsuffix"

	"fmt"

	"io/ioutil"

	"regexp"

	"net/http/cookiejar"

	"encoding/xml"

	vechatsync "github.com/ibigbug/vechat-bot"
	"github.com/ibigbug/vechat-bot/middlewares"
	qrcode "github.com/skip2/go-qrcode"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {

	t, err := template.ParseFiles("templates/index.html")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(r.Context().Value(middlewares.CtxKey("user")))
	locals := map[string]interface{}{
		"qrcode": "/qrcode",
		"user":   r.Context().Value(middlewares.CtxKey("user")),
	}
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, locals)

}

func QRCodeHandler(w http.ResponseWriter, r *http.Request) {
	uuid := vechatsync.GetUUID()
	var png []byte
	png, err := qrcode.Encode(fmt.Sprintf("https://login.weixin.qq.com/l/%s", uuid), qrcode.Medium, 256)
	if err != nil {
		fmt.Println(err)
	}
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)

	go func() {
		jar, _ := cookiejar.New(&cookiejar.Options{
			PublicSuffixList: publicsuffix.List,
		})
		client := &http.Client{
			Jar: jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		checkLoginURL := vechatsync.GetCheckLoinURL(uuid)
		for {
			break
			fmt.Println("Polling url", checkLoginURL.String())
			res, err := http.Get(checkLoginURL.String())

			defer res.Body.Close()
			if err != nil {
				fmt.Println(err)
				continue
			}
			bs, err := ioutil.ReadAll(res.Body)
			if match, _ := regexp.Match("^window\\.code=(400|201)", bs); match {
				continue
			}
			if match, _ := regexp.Match("^window\\.code=200", bs); match {
				// get the redirect uri
				redirectUri := string(vechatsync.GetRedirectURL(bs))
				fmt.Println("Found redirect_uri", redirectUri)
				// login
				rv, _ := client.Get(redirectUri)
				defer rv.Body.Close()
				rvbs, _ := ioutil.ReadAll(rv.Body)
				logonRes := vechatsync.LogonResponse{}
				err := xml.Unmarshal(rvbs, &logonRes)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Printf("%v\n", logonRes)
				initRes := vechatsync.InitClient(client, &logonRes)

				go vechatsync.StartSyncCheck(client, initRes, &logonRes)

				break
			}
			fmt.Println("Still polling.. sth wrong might happend...")
		}

	}()
}
