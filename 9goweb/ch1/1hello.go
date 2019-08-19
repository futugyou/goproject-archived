package main

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
)

func handler(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "hello, %s!", request.URL.Path[1:])
}
func handler1(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "hello1, %s!", request.URL.Path[1:])
}

func log(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name
		fmt.Println("handler funcation called - ", name)
		h(w, r)
	}
}
func main() {
	http.HandleFunc("/sss/", handler1)
	http.HandleFunc("/", log(handler))
	http.ListenAndServe(":8080", nil)
}
