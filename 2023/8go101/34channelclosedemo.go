package main

import (
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(0)

	const Max = 100000
	const NumReceivers = 10
	const NumSenders = 1000

	wg := sync.WaitGroup{}
	wg.Add(NumReceivers)

	data := make(chan int, 100)
	stop := make(chan struct{})

	tostop := make(chan string, 1)
	var stoppedBy string

	go func() {
		stoppedBy = <-tostop
		close(stop)
	}()

	for i := 0; i < NumSenders; i++ {
		go func(id string) {
			for {
				value := rand.Intn(Max)
				if value == 0 {
					select {
					case tostop <- "sender#" + id:
					default:
					}
					return
				}

				select {
				case <-stop:
					return
				default:
				}

				select {
				case <-stop:
					return
				case data <- value:
				}
			}
		}(strconv.Itoa(i))
	}

	for i := 0; i < NumReceivers; i++ {
		go func(id string) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}

				select {
				case <-stop:
					return
				case value := <-data:
					if value == Max-1 {
						select {
						case tostop <- "receive#" + id:
						default:
						}
						return
					}
					log.Println(value)
				}
			}
		}(strconv.Itoa(i))
	}

	wg.Wait()
	log.Println("stopped by" + stoppedBy)
}
