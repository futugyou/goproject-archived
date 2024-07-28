package main

import (
	"errors"
	"fmt"
)

func f1(args int) (int, error) {
	if args == 1 {
		return -1, errors.New("error 1")
	}
	return args + 1, nil
}

type argError struct {
	arg  int
	prob string
}

func (e *argError) Error() string {
	return fmt.Sprintf("%d  -  %s", e.arg, e.prob)
}

func f2(arg int) (int, error) {
	if arg == 1 {
		return -1, &argError{arg, "error 1"}
	}
	return arg + 1, nil
}

func main() {
	for _, i := range []int{9, 8, 1} {
		if r, e := f1(i); e != nil {
			fmt.Println("f1 failed:", e)
		} else {
			fmt.Println("f1 work:", r)
		}
	}
	for _, i := range []int{9, 8, 1} {
		if r, e := f2(i); e != nil {
			fmt.Println("f2 failed:", e)
		} else {
			fmt.Println("f2 work:", r)
		}
	}
	_, e := f2(1)
	if ae, ok := e.(*argError); ok {
		fmt.Println(ae.arg)
		fmt.Println(ae.prob)
	}
}
