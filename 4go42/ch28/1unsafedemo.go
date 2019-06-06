package main

import (
	"fmt"
	"unsafe"
)

type V struct {
	b byte
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
	fmt.Println(unsafe.Sizeof(byte(0)))
	fmt.Println(unsafe.Sizeof(int32(0)))
	fmt.Println(unsafe.Sizeof(int64(0)))

	//var v *V = &V{199, 299}
	var v *V = new(V)
	fmt.Printf("size=%d\n", unsafe.Sizeof(*v))
	var i *int32 = (*int32)(unsafe.Pointer(uintptr(unsafe.Pointer(v)) + uintptr(4*unsafe.Sizeof(byte(0)))))
	fmt.Println("pointer i :", i)
	fmt.Println("pointer uintptr value :", uintptr(unsafe.Pointer(i)))
	*i = int32(100)

	var j *int64 = (*int64)(unsafe.Pointer(uintptr(unsafe.Pointer(v)) + uintptr(unsafe.Sizeof(int64(0)))))
	*j = int64(999)
	fmt.Println("pointer uintptr value :", uintptr(unsafe.Pointer(&v.b)))
	fmt.Println("pointer uintptr value :", uintptr(unsafe.Pointer(&v.i)))
	fmt.Println("pointer uintptr value :", uintptr(unsafe.Pointer(&v.j)))
	v.GetI()
	v.GetJ()
}
