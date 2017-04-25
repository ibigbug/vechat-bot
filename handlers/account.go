package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ibigbug/vechat-bot/models"

	"encoding/json"

	"github.com/go-pg/pg"
	"golang.org/x/oauth2"
)

var conf = &oauth2.Config{
	ClientID:     "744041625955-mp4og7ool9eqc0nnanoah03oj918a5fu.apps.googleusercontent.com",
	ClientSecret: "l0MQW9Y2yelsWnVnrt_GeA9h",
	Scopes:       []string{"openid", "email"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://www.googleapis.com/oauth2/v4/token",
	},
	RedirectURL: "http://dev:5000/account/callback",
}

func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func LoginCallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	code := r.URL.Query().Get("code")
	tok, _ := conf.Exchange(ctx, code)
	client := conf.Client(ctx, tok)
	rv, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err == nil {
		defer rv.Body.Close()
	}

	decoder := json.NewDecoder(rv.Body)
	var account models.GoogleAccount
	err = decoder.Decode(&account)

	var exist models.GoogleAccount
	err = models.Engine.Model(&exist).
		Where("email = ?", account.Email).
		Select()

	if err == nil {
	} else if err == pg.ErrNoRows {
		account.AccessToken = tok.AccessToken
		account.RefreshToken = tok.RefreshToken
		models.Engine.Insert(&account)
	} else {
		panic(err)
	}

	cookie := &http.Cookie{
		Name:     "vsync-jwt",
		Value:    tok.Extra("id_token").(string),
		HttpOnly: false,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
