package main

import (
	"fmt"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

func main() {
	c, _, err := zk.Connect([]string{"127.0.0.1"}, time.Second)
	if err != nil {
		panic(err)
	}
	l := zk.NewLock(c, "/lock", zk.WorldACL(zk.PermAll))
	err := l.Lock()
	if err != nil {
		panic(err)
	}
	fmt.Println("lock success, do your business logic")

	time.Sleep(time.Second * 10)
	l.Unlock()
	fmt.Println("unlock success, finish business logic")
}
