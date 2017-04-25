package middlewares

import (
	"context"
	"log"
	"math/big"
	"net/http"

	"encoding/base64"

	"bytes"

	"encoding/binary"

	"crypto/rsa"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/ibigbug/vechat-bot/data"
	"github.com/ibigbug/vechat-bot/models"
)

type CtxKey string

type Adapter func(http.Handler) http.Handler

func Middleware(h http.Handler, adapters ...Adapter) http.Handler {
	for _, adapter := range adapters {
		h = adapter(h)
	}
	return h
}

func CurrentUser() Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie("vsync-jwt"); err == nil {
				token, _ := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
					for _, key := range data.KeySet {
						if token.Header["kid"] == key.Kid {
							decN, err := base64.RawURLEncoding.DecodeString(key.N)
							if err != nil {
								panic(err)
							}
							n := big.NewInt(0)
							n.SetBytes(decN)

							decE, err := base64.RawURLEncoding.DecodeString(key.E)
							if err != nil {
								panic(err)
							}
							var eBytes []byte
							if len(decE) < 8 {
								eBytes = make([]byte, 8-len(decE), 8)
								eBytes = append(eBytes, decE...)
							} else {
								eBytes = decE
							}
							eReader := bytes.NewReader(eBytes)
							var e uint64
							err = binary.Read(eReader, binary.BigEndian, &e)
							if err != nil {
								panic(e)
							}
							pk := rsa.PublicKey{
								N: n,
								E: int(e),
							}
							return &pk, nil
						}
					}
					return nil, jwt.ErrInvalidKey
				})
				if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
					var user models.GoogleAccount
					if err := models.Engine.Model(&user).
						Where("email = ?", claims["email"]).Select(); err == nil {
						r = r.WithContext(context.WithValue(context.Background(), CtxKey("user"), user))
					}
				} else {
					log.Printf("verify token err, ok: %v, valid: %v", ok, token.Valid)
				}
			}
			h.ServeHTTP(w, r)
		})
	}
}
