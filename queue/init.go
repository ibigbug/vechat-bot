package queue

import "log"
import "os"

var MessageSwitcher = &messageSwitcher{
	queue:   make(chan *Message),
	Workers: make(map[MessageType][]Terminal),
}

var logger = log.New(os.Stdout, "[queue]", log.LstdFlags)

func init() {
	go MessageSwitcher.Start()
}
