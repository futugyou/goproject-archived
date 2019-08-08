package main

import (
	"fmt"
	"sync/atomic"
)

func main() {
	var (
		n uint64 = 97
		m uint64 = 1
		k int    = 2
	)

	const (
		a        = 3
		b uint64 = 4
		c uint32 = 5
		d int    = 6
	)

	show := fmt.Println
	atomic.AddUint64(&n, -m)
	show(n)
	atomic.AddUint64(&n, -uint64(k))
	show(n)
	atomic.AddUint64(&n, ^uint64(a-1))
	show(n)
	show(^uint64(a - 1))
	atomic.AddUint64(&n, ^(b - 1))
	show(n)
	show(b, ^(b - 1))
	atomic.AddUint64(&n, ^uint64(c-1))
	show(n)
	atomic.AddUint64(&n, ^uint64(d-1))
	show(n)
	x := b
	atomic.AddUint64(&n, -x)
	show(n)
	atomic.AddUint64(&n, ^(m - 1))
	show(n)
	atomic.AddUint64(&n, ^uint64(k-1))
	show(n)

	var nn int64 = 123
	var old = atomic.SwapInt64(&nn, 789)
	show(nn, old)
	swapped := atomic.CompareAndSwapInt64(&nn, 123, 456)
	show(swapped, nn)
	swapped = atomic.CompareAndSwapInt64(&nn, 789, 45556)
	show(swapped, nn)
}
