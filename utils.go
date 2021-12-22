package main

import (
	"bufio"
	"io"
	"log"
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
		log.Print("EOF")
	}()
	return recvch
}
