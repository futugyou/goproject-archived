// package main

// //void SayHello(char* s);
// import "C"
// import (
// 	"fmt"
// )

// //export SayHello
// func SayHello(s *C.char){
// 	fmt.Print(C.GoString(s))
// }

// func main(){
// 	C.SayHello(C.CString("hello world"))
// }

package main

//void SayHello(_GoString_ s);
import "C"
import (
	"fmt"
)

//export SayHello
func SayHello(s string) {
	fmt.Print(s)
}
func main() {
	C.SayHello("hello world")
}
