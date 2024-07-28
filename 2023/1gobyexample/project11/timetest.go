package main

import (
	"fmt"
	"time"
)

func main() {
	time1 := time.NewTimer(time.Second * 2)
	<-time1.C
	fmt.Println("time1 expired")
	time2 := time.NewTimer(time.Second)
	go func() {
		<-time2.C
		fmt.Println("time2 expired")
	}()
	stop2 := time2.Stop()
	if stop2 {
		fmt.Println("time2 stop")
	}

	ticker := time.NewTicker(700 * time.Millisecond)
	go func() {
		for t := range ticker.C {
			fmt.Println("tick at : ", t)
		}
	}()

	time.Sleep(time.Millisecond * 1600)
	ticker.Stop()
	fmt.Println("tick stop")
}
