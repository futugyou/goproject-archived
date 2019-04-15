package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

func main() {
	p := fmt.Println
	p(rand.Intn(100), rand.Intn(100))
	p(rand.Float64()*5+5, rand.Float64()*5+5)

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	p(r1.Intn(100), r1.Intn(100))

	s2 := rand.NewSource(42)
	r2 := rand.New(s2)
	p(r2.Intn(100), r2.Intn(100))

	s3 := rand.NewSource(42)
	r3 := rand.New(s3)
	p(r3.Intn(100), r2.Intn(100))

	f, _ := strconv.ParseFloat("123.345", 64)
	p(f)
	i, _ := strconv.ParseInt("0x1c8", 0, 64)
	p(i)
	u, _ := strconv.ParseUint("0x1c8", 0, 64)
	p(u)
	k, _ := strconv.Atoi("135")
	p(k)
}
