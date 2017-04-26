package main

import (
	"flag"

	"github.com/ibigbug/vechat-bot/config"
)

func main() {
	var server = flag.String("server", config.ServerAddr, "Server Listen Addr")
	var db = flag.Bool("syncdb", false, "Syncdb")

	flag.Parse()

	if *db {
		createTable()
	} else {
		runServer(*server)
	}
}
