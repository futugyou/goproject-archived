package main

import (
	"context"
	"fmt"
)

func GenerateNatural(ctx context.Context) chan int {
	ch := make(chan int)
	go func() {
		for i := 2; ; i++ {
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func PrimeFilter(ctx context.Context, in <-chan int, prime int) chan int {
	out := make(chan int)
	go func() {
		for {
			if i := <-in; i%prime != 0 {
				select {
				case out <- i:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
func main() {
	ctx, cancel := context.WithCancel(context.Background())

	ch := GenerateNatural(ctx)
	for i := 0; i < 100; i++ {
		prime := <-ch
		fmt.Printf("%v : %v\n", i+1, prime)
		ch = PrimeFilter(ctx, ch, prime)
	}
	cancel()
}