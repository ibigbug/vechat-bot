package main

import (
	"flag"
)

func main() {
	var server = flag.String("server", DefaultAddr, "Server Listen Addr")
	var db = flag.Bool("syncdb", false, "Syncdb")

	flag.Parse()

	if *db {
		createTable()
	} else {
		runServer(*server)
	}
}
