package main

import (
	"fmt"
	"time"
)

var capacity = 2
var tokenBucket = make(chan struct{}, capacity)
func main() {
	var fillInterval = time.Millisecond * 10

	fillToken := func() {
		ticker := time.NewTicker(fillInterval)
		for {
			select {
			case <-ticker.C:
				select {
				case tokenBucket <- struct{}{}:
				default:
				}
				fmt.Println("current token cnt:", len(tokenBucket), time.Now())
			}
		}
	}

	go fillToken()	 
	time.Sleep(time.Hour)
}


func TakeAvailable(block bool)bool{
	var takenResult bool
	if block{
		select {
		case <-tokenBucket:
			takenResult=true
		}
	}else{
		select{
		case <-tokenBucket:
			takenResult=true
		default:
			takenResult=false
		}
	}
	return takenResult
}