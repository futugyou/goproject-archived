package main

//static const char* cs = "hello";
import "C"
import "./cgo_helper"

///这代码不能正常运行，因为类型不一致
func main() {
	cgo_helper.PrintCString(C.cs)
}
