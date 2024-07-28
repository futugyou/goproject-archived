package main

import (
	"fmt"
	"time"
)

func send(c chan<- int) {
	for i := 0; i < 10; i++ {
		fmt.Println("send ready", i)
		c <- i
		fmt.Println("send ", i)
	}
}
func recv(c <-chan int) {
	for i := range c {
		fmt.Println("received ", i)
	}
}

func main() {
	c := make(chan int, 10)
	go send(c)
	go recv(c)
	time.Sleep(time.Second * 3)
	close(c)
}
