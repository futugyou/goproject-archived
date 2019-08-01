package main

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unsafe"
)

func stringtobyteslice(str string) (bs []byte) {
	strHdr := (*reflect.StringHeader)(unsafe.Pointer(&str))
	sliceHdr := (*reflect.SliceHeader)(unsafe.Pointer(&bs))
	sliceHdr.Data = strHdr.Data
	sliceHdr.Len = strHdr.Len
	sliceHdr.Cap = strHdr.Len
	runtime.KeepAlive(&str)
	return
}

func main() {
	str := strings.Join([]string{"a", "v", "b"}, "") //panic str:="avb"
	s := stringtobyteslice(str)
	fmt.Printf("%s\n", s)
	s[2] = 's'
	fmt.Println(str)
}
