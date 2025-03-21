package main

import (
	"fmt"
	"time"
)

func main() {
	c := make(chan int, 0)
	go func(ch chan<- int, x int) {
		time.Sleep(time.Second)
		ch <- x * x
	}(c, 3)
	done := make(chan struct{})
	go func(ch <-chan int) {
		n := <-ch
		fmt.Println(n)
		time.Sleep(time.Second)
		done <- struct{}{}
	}(c)
	<-done
	fmt.Println("done")
}
