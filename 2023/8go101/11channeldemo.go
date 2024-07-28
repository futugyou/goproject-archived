package main

import (
	"fmt"
	"time"
)

func main() {
	var ball = make(chan string)
	kick := func(name string) {
		for {
			fmt.Println(<-ball, "pass", "\n")
			time.Sleep(time.Second)
			ball <- name
		}
	}
	go kick("1")
	go kick("2")
	go kick("3")
	go kick("4")
	go kick("5")
	go kick("6")
	ball <- "begin"
	var c chan int
	<-c
}
