package main

import (
	"fmt"
	"strconv"
)

func main() {
	jobs := make(chan string, 5)
	done := make(chan bool)
	go func() {
		for {
			j, more := <-jobs
			if more {
				fmt.Println("get job : ", j)
			} else {
				fmt.Println("allready get all")
				done <- true
				return
			}
		}
	}()

	for j := 0; j < 3; j++ {
		jobs <- strconv.Itoa(j)
		fmt.Println("sent job", j)
	}
	close(jobs)
	fmt.Println("sent all")
	<-done

	queue := make(chan string, 5)
	queue <- "one"
	queue <- "two"
	close(queue)
	for item := range queue {
		fmt.Println(item)
	}
}
