package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"time"
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

func headers(w http.ResponseWriter, r *http.Request) {
	h := r.Header
	fmt.Fprintln(w, h)
}

func body(w http.ResponseWriter, r *http.Request) {
	length := r.ContentLength
	body := make([]byte, length)
	r.Body.Read(body)
	fmt.Fprintln(w, string(body))
}

func form1(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Fprintln(w, r.PostForm)
}
func form2(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(1024)
	fmt.Fprintln(w, r.MultipartForm)
}

func redirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "https://www.cnblogs.com/")
	w.WriteHeader(302)
}

func jsonHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	post := &Post{
		User:    "THIS",
		Threads: []string{"one", "three"},
	}
	jsonString, _ := json.Marshal(post)
	w.Write(jsonString)
}

func setcookie(w http.ResponseWriter, r *http.Request) {
	h, _ := time.ParseDuration("8h10m")
	c1 := http.Cookie{
		Name:     "a",
		Value:    "b",
		HttpOnly: true,
		Expires:  time.Now().Add(h),
	}
	c2 := http.Cookie{
		Name:     "f",
		Value:    "m",
		HttpOnly: true,
		Expires:  time.Now().Add(h),
	}

	//w.Header().Set("Set-Cookie", c1.String())
	//w.Header().Add("Set-Cookie", c2.String())
	http.SetCookie(w, &c1)
	http.SetCookie(w, &c2)
}

func getcookie(w http.ResponseWriter, r *http.Request) {
	h := r.Header["Cookie"]
	fmt.Fprintln(w, h)

	cl, err := r.Cookie("a")
	if err != nil {
		fmt.Fprintln(w, err)
	}
	cs := r.Cookies()
	fmt.Fprintln(w, cl)
	fmt.Fprintln(w, cs)
}

type Post struct {
	User    string
	Threads []string
}

func main() {
	http.HandleFunc("/sss/", handler1)
	http.HandleFunc("/", log(handler))
	http.HandleFunc("/header/", headers)
	http.HandleFunc("/body/", body)
	http.HandleFunc("/form1/", form1)
	http.HandleFunc("/form2/", form2)
	http.HandleFunc("/redirect/", redirect)
	http.HandleFunc("/json/", jsonHandle)
	http.HandleFunc("/setcookie/", setcookie)
	http.HandleFunc("/getcookie/", getcookie)
	http.ListenAndServe(":8080", nil)
}
