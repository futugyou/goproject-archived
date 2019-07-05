// $ cd ./number
// $ gcc -c -o number.o number.c
// $ ar rcs libnumber.a number.o   静态库
// gcc -shared -o libnumber.so number.c 动态库

package main

//#cgo CFLAGS: -I./number
//#cgo LDFLAGS: -L${SRCDIR}/number -lnumber
//
//#include "number.h"
import "C"
import (
	"fmt"
)

func main() {
	fmt.Println(C.number_add_mod(10, 9, 8))
}
