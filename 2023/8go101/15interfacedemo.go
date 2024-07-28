package main

import (
	"fmt"
)

type Filter interface {
	About() string
	Process([]int) []int
}

type UniqueFilter struct{}

func (UniqueFilter) About() string {
	return "step one"
}
func (UniqueFilter) Process(input []int) []int {
	outs := make([]int, 0, len(input))
	pushed := make(map[int]bool)
	for _, n := range input {
		if !pushed[n] {
			pushed[n] = true
			outs = append(outs, n)
		}
	}
	return outs
}

type MultipleFilter int

func (mf MultipleFilter) About() string {
	return fmt.Sprintf("keep %v ", mf)
}
func (mf MultipleFilter) Process(input []int) []int {
	outs := make([]int, 0, len(input))
	for _, n := range input {
		if n%int(mf) == 0 {
			outs = append(outs, n)
		}
	}
	return outs
}

func filterAndPrint(filter Filter, input []int) []int {
	filtered := filter.Process(input)
	fmt.Println(filter.About()+":\n\t", filtered)
	return filtered
}
func main() {
	n := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 1, 2, 3, 4, 5, 6, 7, 9}
	fmt.Println(n)
	filters := []Filter{
		UniqueFilter{},
		MultipleFilter(2),
		MultipleFilter(3),
	}
	for _, filter := range filters {
		n = filterAndPrint(filter, n)
	}
	words := []string{
		"Go", "is", "a", "high",
		"efficient", "language.",
	}
	fmt.Println(words)
	//fmt.Println(words...)cannot use words (type []string) as type []interface {}

}
