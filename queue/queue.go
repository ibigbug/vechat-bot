package queue

import (
	"log"
)

type messageSwitcher struct {
	queue chan *Message

	// No need to lock while adding new workers
	Workers map[MessageType][]Terminal
}

func (s *messageSwitcher) BroadCast(msg *Message) {
	s.queue <- msg
}

func (s *messageSwitcher) Register(mt MessageType, term Terminal) {

	if l, ok := s.Workers[mt]; ok {
		s.Workers[mt] = append(l, term)
	} else {
		s.Workers[mt] = []Terminal{term}
	}
}

func (s *messageSwitcher) Start() {
	for {
		select {
		case msg := <-s.queue:
			if msg.ToType == TypeShutdown {
				log.Println("Shutting down")
				return
			} else if l, ok := s.Workers[msg.ToType]; ok {
				for _, term := range l {
					err := term.Notify(msg)
					if err != nil {
						log.Println("Error notify")
					}
				}
			} else {
				log.Println("No handler for MessageType:", msg.ToType)
			}
		}
	}
}
