package main

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(0)

	const Max = 1000
	const NumReceivers = 100

	wg := sync.WaitGroup{}
	wg.Add(NumReceivers)

	data := make(chan int, 100)

	go func() {
		for {
			if value := rand.Intn(Max); value == 0 {
				close(data)
				return
			} else {
				data <- value
			}
		}
	}()

	for i := 0; i < NumReceivers; i++ {
		go func(i int) {
			defer wg.Done()
			for value := range data {
				log.Println(i, value)
			}
		}(i)
	}

	wg.Wait()
}
