package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	ch := make(chan int, 1)
	for i := 0; i < 10; i++ {
		select {
		case ch <- i:
		case x := <-ch:
			fmt.Println(x)
		}
	}

	fmt.Println("Commencing countdown.")
	abort := make(chan struct{})
	go func() {
		os.Stdin.Read(make([]byte, 1))
		abort <- struct{}{}
	}()

	ticker := time.NewTicker(1 * time.Second)
	for countdown := 10; countdown > 0; countdown-- {
		fmt.Println(countdown)
		select {
		case <-ticker.C:
		case <-abort:
			ticker.Stop()
			fmt.Println("launch aborted")
			return
			//default:
		}
	}
	fmt.Println("launched")
}
