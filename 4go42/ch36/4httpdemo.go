package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func index(w http.ResponseWriter, r *http.Request) {
	tm := time.Now().Format(time.RFC1123)
	w.Header().Set("Content-Type", "text/html")
	html := `<doctype html>
        <html>
        <head>
		  <title>Hello World</title>
        </head>
        <body>
        <p>
          Welcome
		</p>		
		<h1>` + tm + `</h1>
        </body>
		</html>`
	fmt.Fprintln(w, html)
	w.Write([]byte("the time is :" + tm))
}

func middlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//logic before handler
		next.ServeHTTP(w, r)
		//logic after handler
	})
}

func loggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("start %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
		log.Printf("completed %s in %v", r.URL.Path, time.Since(start))
	})
}

func main() {
	http.Handle("/", loggingHandler(http.HandlerFunc(index)))
	//http.Handle("/", http.FileServer(http.Dir("C:/inetpub/wwwroot/")))//静态站点
	http.ListenAndServe(":8000", nil)
}
