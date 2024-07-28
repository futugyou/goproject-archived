package main

import (
	"fmt"
)

func False() bool {
	return false
}

func main() {
	//1 格式化后就是2
	switch False()
	{
	case true:
		fmt.Println("true")
	case false:
		fmt.Println("false")
	}

	//2    
	switch False(); {
	case true:
		fmt.Println("true")
	case false:
		fmt.Println("false")
	}

	//3
	switch False() {
	case true:
		fmt.Println("true")
	case false:
		fmt.Println("false")
	}

}
