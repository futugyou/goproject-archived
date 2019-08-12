package main

import (
	"fmt"
	"runtime"
)

func main() {
	c := make(chan int)
	go func() {
		defer func() {
			fmt.Println("4")
			c <- 1
		}()
		defer fmt.Println("3")
		func() {
			defer fmt.Println("2")
			runtime.Goexit()
		}()
		fmt.Println("1")
	}()
	<-c
}
