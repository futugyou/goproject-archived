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

	const Max = 100000
	const NumSenders = 1000

	wg := sync.WaitGroup{}
	wg.Add(1)
	data := make(chan int, 100)
	stop := make(chan struct{})
	for i := 0; i < NumSenders; i++ {
		go func(i int) {
			for {
				select {
				case <-stop:
					//panic(1)
					//log.Println("stop", i)
					//close(data)
					return
				default:
				}
				log.Println("send:", i)
				select {
				case <-stop:
					//panic(2)
					//log.Println("stop", i)
					//close(data)
					return
				case data <- rand.Intn(Max):
				}
			}
		}(i)
	}

	go func() {
		defer wg.Done()
		for value := range data {
			log.Println("receiver:", value)
			if value == Max-1 {
				close(stop)
				return
			}
		}
	}()

	wg.Wait()
}
