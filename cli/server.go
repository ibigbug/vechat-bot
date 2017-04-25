package main

import (
	"context"
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
		middlewares.CurrentUser(context.Background()),
	))
	mux.HandleFunc("/qrcode", handlers.QRCodeHandler)

	// telegram
	mux.Handle("/telegram", middlewares.Middleware(
		http.HandlerFunc(handlers.AddTelegramBotHandler),
		middlewares.CurrentUser(context.Background()),
	))
	mux.Handle("/telegram/toggle", middlewares.Middleware(
		http.HandlerFunc(handlers.ToggleBotStatusHandler),
		middlewares.CurrentUser(context.Background()),
	))

	// account
	mux.Handle("/account/login", middlewares.Middleware(
		http.HandlerFunc(handlers.LoginPageHandler),
		middlewares.CurrentUser(context.Background()),
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