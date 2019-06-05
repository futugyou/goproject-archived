package main

import (
	"fmt"
	"sync"
	"time"
)

var m *sync.Mutex

func main() {
	wg := sync.WaitGroup{}
	wg.Add(20)
	var rwMutex sync.RWMutex
	Data := 0
	for i := 0; i < 10; i++ {
		go func(t int) {
			rwMutex.RLock()
			defer rwMutex.RUnlock()
			fmt.Printf("read data : %v\n", Data)
			wg.Done()
			time.Sleep(2 * time.Second)
		}(i)

		go func(t int) {
			rwMutex.Lock()
			defer rwMutex.Unlock()
			Data += t
			fmt.Printf("write data :%v %d \n", Data, t)
			wg.Done()

			time.Sleep(2 * time.Second)
		}(i)
	}
	time.Sleep(5 * time.Second)
	wg.Wait()
}
