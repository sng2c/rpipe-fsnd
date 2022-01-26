package main

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"io"
)

func RecvChannel(rd io.Reader) <-chan string {
	recvch := make(chan string)
	go func() {
		defer close(recvch)
		scanner := bufio.NewScanner(rd)
		for scanner.Scan() {
			var line = scanner.Text()
			recvch <- line
		}
		log.Debug("EOF")
	}()
	return recvch
}
