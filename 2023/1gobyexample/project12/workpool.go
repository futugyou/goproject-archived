package main

import (
	"fmt"
	"time"
)

func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		fmt.Println("work:", id, " process job :", j)
		//time.Sleep(time.Second)
		results <- j * 2
	}
}
func main() {
	jobs := make(chan int, 100)
	results := make(chan int, 100)
	for i := 0; i < 3; i++ {
		go worker(i, jobs, results)
	}
	for j := 0; j < 8; j++ {
		jobs <- j
	}

	close(jobs)
	for j := 0; j < 8; j++ {
		<-results
	}

	//////base rate-limiter
	requests := make(chan int, 5)
	for i := 0; i < 5; i++ {
		requests <- i
	}
	close(requests)

	limiter := time.Tick(time.Millisecond * 100)
	for req := range requests {
		<-limiter
		fmt.Println("request id: ", req)
	}

	/////buffer limiter
	burstyLimiter := make(chan time.Time, 3)
	for i := 0; i < 3; i++ {
		burstyLimiter <- time.Now()
	}

	go func() {
		for t := range time.Tick(time.Millisecond * 200) {
			burstyLimiter <- t
		}
	}()

	burstyRequests := make(chan int, 5)
	for i := 0; i < 5; i++ {
		burstyRequests <- i
	}
	close(burstyRequests)
	for req := range burstyRequests {
		<-burstyLimiter
		fmt.Println("request", req, time.Now())
	} 
}
