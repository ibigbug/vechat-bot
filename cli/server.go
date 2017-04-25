package main

import (
	"context"
	"net/http"

	"github.com/ibigbug/vechat-bot/handlers"
	"github.com/ibigbug/vechat-bot/middlewares"
)

const (
	DefaultAddr = ":5000"
)

func runServer(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/", middlewares.Middleware(
		http.HandlerFunc(handlers.IndexHandler),
		middlewares.CurrentUser(context.Background()),
	))

	mux.HandleFunc("/qrcode", handlers.QRCodeHandler)
	mux.HandleFunc("/account/login", handlers.LoginPageHandler)
	mux.HandleFunc("/account/callback", handlers.LoginCallbackHandler)

	srv := http.Server{
		Addr:    addr,
		Handler: mux,
	}
	srv.ListenAndServe()
}
