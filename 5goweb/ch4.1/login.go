package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

func sayhelloName(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Println(r.Form)
	fmt.Println("path", r.URL.Path)
	fmt.Println("scheme", r.URL.Scheme)
	fmt.Println(r.Form["url_long"])
	for k, v := range r.Form {
		fmt.Println("key:", k, "   value:", strings.Join(v, ""))
	}
	fmt.Fprintf(w, "hello world")
}

func login(w http.ResponseWriter, r *http.Request) {
	fmt.Println("method:", r.Method)
	if r.Method == "GET" {
		t, _ := template.ParseFiles("login.gtpl")
		log.Println(t.Execute(w, nil))
	} else {
		r.ParseForm()
		name := r.Form.Get("username")
		if len(name)==0{
			fmt.Fprintf(w, "name can not be null")
		}

		slice:=[]string{"apple","pear","banana"}

		v := r.Form.Get("fruit")
		var fruitcheck = false;
		for _, item := range slice {
			if item == v {
				fruitcheck = true
			}
		}
		if !fruitcheck{
			fmt.Fprintf(w, "fruit can not be null")
		}
		fmt.Println("name:", name)
		fmt.Println("pass", r.FormValue("password"))
		fmt.Println("fruit", v)
	}
}

func main() {
	http.HandleFunc("/", sayhelloName)
	http.HandleFunc("/login", login)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("listenandserve:", err)
	}
}
