package main

import (
	"fmt"
	"os"
	"sort"
)

func main() {
	strs := []string{"c", "a", "b"}
	sort.Strings(strs)
	fmt.Println("strings:", strs)

	ints := []int{7, 6, 4, 3, 5, 4, 4, 5}
	s := sort.IntsAreSorted(ints)
	fmt.Println("sorted:", s)
	sort.Ints(ints)
	fmt.Println(ints)
	s = sort.IntsAreSorted(ints)
	fmt.Println("sorted:", s)

	ss := []string{"zdda", "aaa", "s"}
	sort.Sort(ByLength(ss))

	// ss := ByLength{"zdda", "aaa", "s"}
	// sort.Sort(ss)
	fmt.Println(ss)
	test()
}

// type Interface interface {
// 	// Len is the number of elements in the collection.
// 	Len() int
// 	// Less reports whether the element with
// 	// index i should sort before the element with index j.
// 	Less(i, j int) bool
// 	// Swap swaps the elements with indexes i and j.
// 	Swap(i, j int)
// }

type ByLength []string

func (s ByLength) Len() int {
	return len(s)
}
func (s ByLength) Less(i, j int) bool {
	return len(s[i]) < len(s[j])
}
func (s ByLength) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func test() {
	panic("a problem")

	_, err := os.Create("/tmp/file")
	if err != nil {
		panic(err)
	}
}
