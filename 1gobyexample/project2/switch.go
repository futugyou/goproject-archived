package main

import "fmt"
import "time"

func main()  {
	i:=0
	switch i {
	case 0:
		fmt.Println("0")
	}
	switch time.Now().Weekday() {
	case 1:
		fmt.Println("one")
	case 2:
		fmt.Println("two")
	default:
		fmt.Println("other")
	}
	t:=time.Now()
	fmt.Println(t)
	switch  {
	case t.Hour()<=12:
		fmt.Println("mooning")
	default:
		fmt.Println("afternoon")
	}
}