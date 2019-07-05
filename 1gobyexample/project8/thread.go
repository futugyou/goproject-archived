package main

import "fmt"

func f(from string) {
	for i := 0; i < 3; i++ {
		fmt.Println(from, ":", i)
	}
}

func main() {
	//f("direct")
	go f("thread")

	go func(msg string) {
		fmt.Println(msg)
	}("going")

	// var input string
	// fmt.Scanln(&input)
	// fmt.Println(input)

	message := make(chan mess, 2)
	fmt.Println(message)
	go func() { message <- mess{4, 5} }()
	go func() { message <- mess{1, 2} }()
	message <- mess{5, 6}
	message <- mess{7, 8}

	fmt.Println(<-message)
	fmt.Println(<-message)
	fmt.Println(<-message)
	fmt.Println(<-message)

	pings := make(chan string, 1)
	pongs := make(chan string, 1)
	ping(pings, "ssss")
	pong(pings, pongs)
	fmt.Println(<-pongs)
}

type mess struct {
	a int
	b int
}

func ping(pings chan<- string, msg string) {
	pings <- msg
}
func pong(pings <-chan string, pongs chan<- string) {
	msg := <-pings
	pongs <- msg
}
