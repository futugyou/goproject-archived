package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	const N = 5
	var values [N]int32

	wgA := sync.WaitGroup{}
	wgB := sync.WaitGroup{}
	wgA.Add(N)
	wgB.Add(1)

	for i := 0; i < N; i++ {
		i := i
		go func() {
			wgB.Wait()
			fmt.Println(values[i])
			wgA.Done()
		}()
	}

	for i := 0; i < N; i++ {
		values[i] = 50 + rand.Int31n(50)
	}
	wgB.Done()
	wgA.Wait()
}
