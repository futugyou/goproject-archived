package main

import (
	"context"
	"log"
	"os"
	"time"
)

var logs *log.Logger

func doClearn(ctx context.Context) {
	for {

		time.Sleep(time.Second * 1)
		select {
		case <-ctx.Done():
			logs.Println("doClearn :reslove cancel ,exit now")
			return
		default:
			logs.Println("doClearn : get status times/1second")
		}
	}
}

func doNothing(ctx context.Context) {
	for {
		time.Sleep(time.Second * 3)
		select {
		case <-ctx.Done():
			logs.Println("doNothing : reslove cancel ,but not exit")
		default:
			logs.Println("doNothing : get status times/3second")
		}
	}
}

func main() {
	logs = log.New(os.Stdout, "", log.Ltime)
	ctx, cancel := context.WithCancel(context.Background())

	go doClearn(ctx)
	go doNothing(ctx)

	time.Sleep(20 * time.Second)
	logs.Println("cancel")
	cancel()
	time.Sleep(10 * time.Second)
}
