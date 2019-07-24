package main

import (
	"fmt"
)

func main() {
	langs := map[struct{ dynamic, strong bool }]map[string]int{
		{true, false}:  {"javascript": 1995},
		{false, true}:  {"go": 2009},
		{false, false}: {"c": 1972},
	}

	m0 := map[*struct{ dynamic, strong bool }]*map[string]int{}
	for category, langinfo := range langs {
		m0[&category] = &langinfo
		//no changed
		category.dynamic, category.strong = true, true
		fmt.Println(m0)
	}
	fmt.Println(m0)
	for category, langinfo := range langs {
		fmt.Println(category, langinfo)
	}
	m1 := map[struct{ dynamic, strong bool }]map[string]int{}
	for category, langinfo := range m0 {
		m1[*category] = *langinfo
	}
	fmt.Println(len(m0), len(m1))
	fmt.Println(m1)
}
