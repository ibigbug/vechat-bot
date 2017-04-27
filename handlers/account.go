package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ibigbug/vechat-bot/models"

	"encoding/json"

	"github.com/go-pg/pg"
	"github.com/ibigbug/vechat-bot/config"
	"golang.org/x/oauth2"
)

const (
	UserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
)

var conf = &oauth2.Config{
	ClientID:     "744041625955-mp4og7ool9eqc0nnanoah03oj918a5fu.apps.googleusercontent.com",
	ClientSecret: "l0MQW9Y2yelsWnVnrt_GeA9h",
	Scopes:       []string{"openid", "email"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL: "https://www.googleapis.com/oauth2/v4/token",
	},
	RedirectURL: config.GoogleCallbackURL,
}

func LoginPageHandler(w http.ResponseWriter, r *http.Request) {
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func LoginCallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.TODO()
	code := r.URL.Query().Get("code")
	tok, _ := conf.Exchange(ctx, code)
	client := conf.Client(ctx, tok)
	res, err := client.Get(UserInfoURL)
	if err != nil {
		log.Println("Error getting userinfo", err)
		http.Error(w, "Login Failed. Please try again. Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var account models.GoogleAccount
	err = decoder.Decode(&account)

	var exist models.GoogleAccount
	err = models.Engine.Model(&exist).
		Where("email = ?", account.Email).
		Select()

	if err == pg.ErrNoRows {
		account.AccessToken = tok.AccessToken
		account.RefreshToken = tok.RefreshToken
		models.Engine.Insert(&account)
		log.Printf("insert new user: %s\n", account.Email)
	} else if err != nil {
		panic(err)
	} else {
		exist.AccessToken = tok.AccessToken
		exist.RefreshToken = tok.RefreshToken
		models.Engine.Model(&exist).
			Column("access_token", "refresh_token").
			Update()
		log.Printf("update existed user: %s\n", exist.Email)
	}

	cookie := &http.Cookie{
		Name:     "vsync-secure-cookie",
		Value:    tok.Extra("id_token").(string),
		HttpOnly: false,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}
