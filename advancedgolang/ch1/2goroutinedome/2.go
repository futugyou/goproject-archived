package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var total int32

func worker(wg *sync.WaitGroup) {
	defer wg.Done()
	var i int32
	for i = 0; i <= 100; i++ {
		atomic.AddInt32(&total, i)
	}
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)
	go worker(&wg)
	go worker(&wg)
	wg.Wait()
	fmt.Println(total)
}
