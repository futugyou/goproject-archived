package main

import "fmt"

type student struct {
	Name string
	Age  int
}

func main() {
	m := make(map[string]*student)
	stus := []student{
		{Name: "11", Age: 10},
		{Name: "22", Age: 20},
		{Name: "33", Age: 30},
	}
	// for _, stu := range stus {
	// 	m[stu.Name] = &stu
	// }//map[11:0xc000004460 22:0xc000004460 33:0xc000004460]
	for i := 0; i < len(stus); i++ {
		m[stus[i].Name] = &stus[i]
	} //map[11:0xc000086000 22:0xc000086018 33:0xc000086030]
	fmt.Print(m)
}
