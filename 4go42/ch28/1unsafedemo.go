package main

import (
	"fmt"
	"unsafe"
)

type V struct {
	i int32
	j int64
}

func (v V) GetI() {
	fmt.Printf("i=%d\n", v.i)
}
func (v V) GetJ() {
	fmt.Printf("j=%d\n", v.j)
}
func main() {
	fmt.Println(unsafe.Sizeof(int64(0)))

	var v *V = &V{199, 299}
	var i *int32 = (*int32)(unsafe.Pointer(v))
	*i = int32(100)

	var j *int64 = (*int64)(unsafe.Pointer(uintptr(unsafe.Pointer(v)) + uintptr(unsafe.Sizeof(int64(0)))))
	*j = int64(999)
	v.GetI()
	v.GetJ()
}
