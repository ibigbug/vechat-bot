package handlers

import "golang.org/x/net/websocket"
import "io"

func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}
