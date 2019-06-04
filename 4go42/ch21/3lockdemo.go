package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	wg := sync.WaitGroup{}
	var mutex sync.Mutex
	fmt.Println("Locking (GO)")
	mutex.Lock()
	fmt.Println("locked (GO)")
	wg.Add(3)

	for i := 1; i < 4; i++ {
		go func(i int) {
			fmt.Printf("Locking (G%d)\n", i)
			mutex.Lock()
			fmt.Printf("Locked (G%d)\n", i)

			time.Sleep(time.Second * 2)
			mutex.Unlock()
			fmt.Printf("unlocked (G%d)\n", i)
			wg.Done()
		}(i)
	}

	time.Sleep(time.Second * 5)
	fmt.Println("read unlock (G0)")
	mutex.Unlock()
	fmt.Println("Unlocked (G0)")

	wg.Wait()
}
