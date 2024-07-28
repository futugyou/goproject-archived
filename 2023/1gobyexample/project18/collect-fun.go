package main

import (
	"fmt"
	"strings"
)

func Index(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

func Include(vs []string, t string) bool {
	return Index(vs, t) >= 0
}
func Any(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if f(v) {
			return true
		}
	}
	return false
}

func All(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

func Filter(vs []string, f func(string) bool) []string {
	tmp := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			tmp = append(tmp, v)
		}
	}
	return tmp
}

func Map(vs []string, f func(string) string) []string {
	tmp := make([]string, len(vs))
	for i, v := range vs {
		tmp[i] = f(v)
	}
	return tmp
}

func main() {
	var strs = []string{"aaa", "bbb", "ccc", "ddd"}
	fmt.Println(Index(strs, "bbb"))
	fmt.Println(Include(strs, "ddd"))
	fmt.Println(Any(strs, func(v string) bool {
		return strings.HasPrefix(v, "ccc")
	}))
	fmt.Println(All(strs, func(v string) bool {
		return strings.HasPrefix(v, "bbb")
	}))
	fmt.Println(Filter(strs, func(v string) bool {
		return strings.Contains(v, "b")
	}))
	fmt.Println(Map(strs, strings.ToUpper))
}
