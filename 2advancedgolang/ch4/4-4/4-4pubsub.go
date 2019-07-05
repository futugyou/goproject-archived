package main

import (
	fmt "fmt"
	"strings"
	"time"

	"github.com/docker/docker/pkg/pubsub"
)

func main() {
	p := pubsub.NewPublisher(100*time.Millisecond, 10)

	golang := p.SubscribeTopic(func(v interface{}) bool {
		if key, ok := v.(string); ok {
			if strings.HasPrefix(key, "golang:") {
				return true
			}
		}
		return false
	})
	docker := p.SubscribeTopic(func(v interface{}) bool {
		if key, ok := v.(string); ok {
			if strings.HasPrefix(key, "docker:") {
				return true
			}
		}
		return false
	})

	go p.Publish("hi")
	go p.Publish("golang: https://golang.org")
	go p.Publish("docker: https://www.docker.com")
	time.Sleep(1)
	go func() {
		fmt.Println("golang topic:", <-golang)
	}()

	go func() {
		fmt.Println("docker topic:", <-docker)
	}()
	time.Sleep(10)

	//<-make(chan bool) initiate deadlock!
	//fatal error: all goroutines are asleep - deadlock!
	//<-make(chan bool)
}
