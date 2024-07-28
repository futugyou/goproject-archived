package main

/*
#include <stdint.h>

union B {
	int i;
	float f;
};
enum C{
	ONE,
	TWO,
};
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func main() {
	var b C.union_B
	fmt.Println("b.i:", *(*C.int)(unsafe.Pointer(&b)))
	fmt.Println("b.f:", *(*C.float)(unsafe.Pointer(&b)))

	var c C.enum_C= C.TWO
	fmt.Println(c)
	fmt.Println(C.ONE)
	fmt.Println(C.TWO)
}
