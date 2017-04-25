package main

import (
	"net/http"

	"github.com/ibigbug/vechat-bot/handlers"
	"github.com/ibigbug/vechat-bot/middlewares"
	"golang.org/x/net/websocket"
)

const (
	DefaultAddr = ":5000"
)

func runServer(addr string) {
	mux := http.NewServeMux()

	// index
	mux.Handle("/", middlewares.Middleware(
		http.HandlerFunc(handlers.IndexHandler),
		middlewares.CurrentUser(),
	))
	mux.Handle("/qrcode", middlewares.Middleware(
		http.HandlerFunc(handlers.QRCodeHandler),
		middlewares.CurrentUser(),
	))

	// telegram
	mux.Handle("/telegram", middlewares.Middleware(
		http.HandlerFunc(handlers.AddTelegramBotHandler),
		middlewares.CurrentUser(),
	))
	mux.Handle("/telegram/toggle", middlewares.Middleware(
		http.HandlerFunc(handlers.ToggleBotStatusHandler),
		middlewares.CurrentUser(),
	))

	// wechat
	mux.Handle("/wechat", middlewares.Middleware(
		http.HandlerFunc(handlers.WechatLoginPage),
		middlewares.CurrentUser(),
	))

	// account
	mux.Handle("/account/login", middlewares.Middleware(
		http.HandlerFunc(handlers.LoginPageHandler),
		middlewares.CurrentUser(),
	))
	mux.HandleFunc("/account/callback", handlers.LoginCallbackHandler)

	// websocket
	mux.Handle("/ws", websocket.Handler(handlers.EchoServer))

	srv := http.Server{
		Addr:    addr,
		Handler: mux,
	}
	srv.ListenAndServe()
}
