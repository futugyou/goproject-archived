package main

import (
	"fmt"
	"time"
)

func counting(c chan<- int) {
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		c <- i
	}
	close(c)
}

func main() {
	msg := "starting main"
	fmt.Println(msg)
	bus := make(chan int)
	msg = "starting a gofunc"
	go counting(bus)
	for count := range bus {
		fmt.Println("count:", count)
	}
}

//go build -gcflags "-N -l"   C:\Code\Golang\src\work\golang-test\5goweb\ch11\1debugdemo.go

// gdb  C:\Code\Golang\src\work\golang-test\5goweb\ch11\1debugdemo
