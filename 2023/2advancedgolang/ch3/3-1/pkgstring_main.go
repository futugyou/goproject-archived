package main

import (
	"fmt"
	pkg "work/golang-test/advancedgolang/ch3/3-1/pkgstring"
)

// go tool compile -S C:\Code\Golang\src\work\golang-test\advancedgolang\ch3\3-1\pkgstring\pkg.go
func main() {
	fmt.Println(pkg.Name)
}
