package main

import (
	"fmt"
	"sync"
)

func main() {
	// channelMethod()
	waitMethod()
}

func channelMethod() {
	done := make(chan int, 10)
	for i := 0; i < cap(done); i++ {
		go func() {
			fmt.Println("hello world")
			done <- 1
		}()
	}
	for i := 0; i < cap(done); i++ {
		<-done
	}
}

func waitMethod() {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			fmt.Println("hello world")
			wg.Done()
		}()
	}
	wg.Wait()
}
