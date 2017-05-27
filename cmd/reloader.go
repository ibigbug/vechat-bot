package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

const (
	RUN_IN_RELOADER = "RUN_IN_RELOADER"
)

func NewReloader() *Reloader {
	return &Reloader{
		Name:        "GolangReloader",
		shoudReload: false,
	}
}

type Reloader struct {
	Name        string
	shoudReload bool
}

func (r *Reloader) Run() {

}

func (r *Reloader) triggerReload(fp string) {
	log.Printf("* Detected change in %s, reloading\n", fp)
	os.Exit(3)
}

func (r *Reloader) RestartWithReloader() int {
	for {
		log.Printf(" * Restarting with %s\n", r.Name)
		args := getArgsForReloading()
		cmd := exec.Command(args[0], args[1:]...)
		var ws syscall.WaitStatus
		if err := cmd.Run(); err != nil {
			log.Println(err)
			if exitError, ok := err.(*exec.ExitError); ok {
				ws = exitError.Sys().(syscall.WaitStatus)
			}
		} else {
			ws = cmd.ProcessState.Sys().(syscall.WaitStatus)

		}
		if ws.ExitStatus() != 3 {
			return ws.ExitStatus()
		}
	}
}

func RunWithReloader(f func()) {
	reloader := NewReloader()
	if os.Getenv(RUN_IN_RELOADER) == "true" {
		go f()
		reloader.Run()
	} else {
		os.Exit(reloader.RestartWithReloader())
	}
}

func getArgsForReloading() []string {
	return os.Args
}
