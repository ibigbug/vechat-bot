package data

import (
	"encoding/json"
	"log"
	"net/http"
)

type RsaPubKey struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

const (
	PubkeySetURL = "https://www.googleapis.com/oauth2/v3/certs"
)

var KeySet []RsaPubKey

// Load JWT pubkey set from https://www.googleapis.com/oauth2/v3/certs
func init() {
	if res, err := http.Get(PubkeySetURL); err == nil {
		var result struct {
			Keys []RsaPubKey `json:"keys"`
		}
		if err = json.NewDecoder(res.Body).Decode(&result); err == nil {
			KeySet = result.Keys
			log.Printf("Pubkey loaded..\n")
		} else {
			log.Printf("Error parsing pubkey...%s\n", err)
		}
	} else {
		log.Printf("Error loading pubkey...%s\n", err)
	}
}
