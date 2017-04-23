package main

import (
	"net/http"

	"github.com/ibigbug/vechat-sync/handlers"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handlers.IndexHandler)
	mux.HandleFunc("/qrcode", handlers.QRCodeHandler)

	srv := http.Server{
		Addr:    ":3000",
		Handler: mux,
	}
	srv.ListenAndServe()
}
