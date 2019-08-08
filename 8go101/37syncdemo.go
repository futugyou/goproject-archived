package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	const N = 10
	var values [N]string
	cond := sync.NewCond(&sync.Mutex{})
	cond.L.Lock()

	for i := 0; i < N; i++ {
		d := time.Second * time.Duration(rand.Intn(10)) / 10
		go func(i int) {
			time.Sleep(d)
			fmt.Println("1111:", i)
			cond.L.Lock()
			values[i] = string('a' + i)
			fmt.Println("2222:", i)
			cond.L.Unlock()
			fmt.Println("3333:", i)
			cond.Broadcast()
			fmt.Println("4444:", i)
		}(i)
	}

	checkCondition := func() bool {
		fmt.Println(values)
		for i := 0; i < N; i++ {
			if values[i] == "" {
				return false
			}
		}
		return true
	}
	for !checkCondition() {
		cond.Wait()
	}
	cond.L.Unlock()
}
