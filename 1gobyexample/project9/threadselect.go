package main

import (
	"fmt"
	"time"
)

func main() {
	c1 := make(chan string)
	c2 := make(chan string)

	go func() {
		time.Sleep(time.Second * 3)
		c1 <- "one"
	}()
	go func() {
		time.Sleep(time.Second * 3)
		c2 <- "two"
	}()

	for i := 0; i < 2; i++ {
		select {
		case msg1 := <-c1:
			fmt.Println("get one : ", msg1)
		case msg1 := <-c2:
			fmt.Println("get two : ", msg1)
		}
	}
	//select case can used for timeout cancel.
	//you can remove 'for' from this code and
	//use 'case <-time.After(time.Second * 2)'
	//to set timeout.

	messages := make(chan string)
	signals := make(chan string)

	select {
	case msg := <-messages:
		fmt.Println("message:", msg)
	case sing := <-signals:
		fmt.Println("signals", sing)
	default:
		fmt.Println("default")
	}
	msg := "this is message"
	select {
	case messages <- msg:
		fmt.Println("message:", msg)
	case signals <- msg:
		fmt.Println("signals", msg)
	default:
		fmt.Println("default")
	}
}
