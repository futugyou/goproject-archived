package main

import (
	"fmt"
	"sort"
)

type person struct {
	name string
	age  int
}
type personSlice []person

func (s personSlice) Len() int           { return len(s) }
func (s personSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s personSlice) Less(i, j int) bool { return s[i].age < s[j].age }

func main() {
	a := []int{9, 8, 7, 6, 5, 4, 3, 2}
	sort.Ints(a)
	fmt.Println(a)
	ss := []string{"b", "v", "c", "s", "w"}
	sort.Strings(ss)
	fmt.Println(ss)
	sort.Sort(sort.Reverse(sort.StringSlice(ss)))
	fmt.Println(ss)

	p := personSlice{
		{
			name: "a",
			age:  1,
		}, {
			name: "b",
			age:  2,
		}, {
			name: "c",
			age:  3,
		}, {
			name: "d",
			age:  4,
		}, {
			name: "e",
			age:  5,
		}, {
			name: "f",
			age:  6,
		},
	}

	sort.Sort(p)
	fmt.Println(p)
	sort.Stable(p)
	fmt.Println(p)

	sort.Slice(p, func(i, j int) bool {
		return p[i].age > p[j].age
	})
	fmt.Println(p)
}
