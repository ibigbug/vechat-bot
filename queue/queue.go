package queue

import (
	"log"
)

type messageSwitcher struct {
	queue chan *Message

	// No need to lock while adding new workers
	Workers map[MessageType][]Terminal
}

func (s *messageSwitcher) Broadcast(msg *Message) {
	s.queue <- msg
	log.Println("Broadcasting message:", msg)
}

func (s *messageSwitcher) Register(mt MessageType, term Terminal) {
	log.Println("new worker registed", mt)
	if l, ok := s.Workers[mt]; ok {
		s.Workers[mt] = append(l, term)
	} else {
		s.Workers[mt] = []Terminal{term}
	}
	log.Println("workers after registerd", s.Workers)
}

func (s *messageSwitcher) Start() {
	log.Println("MessageSwitcher started")
	for {
		select {
		case msg := <-s.queue:
			log.Println("Got new msg", msg, "workers:", s.Workers)
			if msg.ToType == TypeShutdown {
				log.Println("Shutting down")
				return
			} else if l, ok := s.Workers[msg.ToType]; ok {
				for _, term := range l {
					err := term.Notify(msg)
					if err != nil {
						log.Println("Error notify", msg)
					} else {
						log.Println("Notified msg", msg)
					}
				}
			} else {
				log.Println("No handler for MessageType:", msg.ToType)
			}
		}
	}
}
