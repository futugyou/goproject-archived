package main

import "fmt"
import "math"

const s string="this is const"

func main()  {
	fmt.Println("Hello golang!")
	fmt.Println("7/4=",7/4)
	fmt.Println("7.0/3.0=",7.0/3.0)
	fmt.Println(true)
	fmt.Println(1^3)
	fmt.Println(2<<4)

	var a string = "this is test"
	fmt.Println(a)

	var b,c int = 3,5
	fmt.Println(b,c)

	var d = true
	fmt.Println(d)

	var e int
	fmt.Println(e)
	f :="abc"
	fmt.Println(f)

	fmt.Println(s)

	const n =500000000
	const m = 3e20/n
	fmt.Println(m)
	fmt.Println(int64(m))

	fmt.Println(math.Sin(n))
	for b < c {
		fmt.Println(b)
		b++
	}
	for{
		fmt.Println("s")
		break
	}
	if index:=9;index<10 {
		fmt.Println("index<10")
	}
}