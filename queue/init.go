package queue

var MessageSwitcher = &messageSwitcher{
	queue:   make(chan *Message),
	Workers: make(map[MessageType][]Terminal),
}

func init() {
	go MessageSwitcher.Start()
}
