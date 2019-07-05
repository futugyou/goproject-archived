package main

import "fmt"

func main() {
	s := make([]string, 3)
	fmt.Println(len(s))
	s[0] = "a"
	s[1] = "b"
	s[2] = "c"
	s = append(s, "d")
	s = append(s, "e", "f")
	fmt.Println(s)

	c := make([]string, len(s))
	copy(c, s)
	fmt.Println(c)

	fmt.Println(c[:4])
	fmt.Println(c[1:])

	m := make(map[string]int)
	m["a"] = 1
	m["b"] = 2
	fmt.Println(m)
	delete(m, "a")
	fmt.Println(m)
	_, prs := m["b"]
	fmt.Println("prs:", prs)

	n := map[string]string{"a": "2", "c": "5"}
	fmt.Println(n)

	for k, v := range n {
		fmt.Printf("%s -> %s\n", k, v)
	}

	for i, d := range "abc" {
		fmt.Println(i, d)
	}
	cc := add(8, 90)
	fmt.Println(cc)

	dd, ee := clonefun(1, 10)
	fmt.Println(dd, ee)

	muns := []int{1, 2, 3, 4, 5, 6}
	cc = add2(muns...)
	fmt.Println(cc)

	a11 := intSeq(2)
	fmt.Println(a11(1))
	fmt.Println(a11(1))
	fmt.Println(a11(1))

	tt := fact(5)
	fmt.Println(tt)
}

func add(a, b int) int {
	return a + b
}

func clonefun(a, b int) (int, int) {
	return b, a
}

func add2(muns ...int) int {
	a := 0
	for _, i := range muns {
		a += i
	}
	return a
}

func intSeq(a int) func(b int) int {
	i := a
	return func(b int) int {
		i += b
		return i
	}
}

func fact(i int) int {
	if i == 0 {
		return 1
	} else {
		return i * fact(i-1)
	}
}
