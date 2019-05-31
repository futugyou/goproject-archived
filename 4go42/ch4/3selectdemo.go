package main

import (
	"fmt"
	"time"
)

func main() {
	var c1, c2, c3 chan int
	var i1, i2 int
	select {
	case i1 = <-c1:
		fmt.Println("get c1 : ", i1)
	case c2 <- i2:
		fmt.Println("send c2")
	case i2, ok := (<-c3):
		if ok {
			fmt.Println("get i2 : ", i2)
		} else {
			fmt.Println("c3 close")
		}
	case <-time.After(time.Second * 3):
		fmt.Println("time out")
	}
}
