package main

import (
	"fmt"
	"os"
	"time"
)

type Ball uint64

func Play(playerName string, table chan Ball) {
	var lastValue Ball = 1
	for {
		ball := <-table
		fmt.Println(playerName, ball)
		ball += lastValue
		if ball < lastValue {
			os.Exit(0)
		}

		lastValue = ball
		table <- ball
		time.Sleep(time.Second)
	}
}

func main() {
	table := make(chan Ball)
	go func() {
		table <- 1
	}()
	go Play("A:", table)
	Play("B:", table)
}
