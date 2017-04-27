package queue

type messageSwitcher struct {
	queue chan *Message

	// No need to lock while adding new workers
	Workers map[MessageType][]Terminal
}

func (s *messageSwitcher) Broadcast(msg *Message) {
	s.queue <- msg
	logger.Println("Broadcasting message:", msg)
}

func (s *messageSwitcher) Register(mt MessageType, term Terminal) {
	logger.Println("new worker registed, type:", mt)
	if l, ok := s.Workers[mt]; ok {
		s.Workers[mt] = append(l, term)
	} else {
		s.Workers[mt] = []Terminal{term}
	}
	logger.Println("workers after registerd", s.Workers)
}

func (s *messageSwitcher) Start() {
	logger.Println("MessageSwitcher started")
	for {
		select {
		case msg := <-s.queue:
			logger.Println("Got new msg", msg, "workers:", s.Workers)
			if msg.ToType == TypeShutdown {
				logger.Println("Shutting down")
				return
			} else if l, ok := s.Workers[msg.ToType]; ok {
				for _, term := range l {
					err := term.Notify(msg)
					if err != nil {
						logger.Println("Error notify", msg)
					} else {
						logger.Println("Notified msg", msg)
					}
				}
			} else {
				logger.Println("No handler for MessageType:", msg.ToType)
			}
		}
	}
}
