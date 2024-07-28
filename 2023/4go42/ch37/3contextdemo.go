package main

import (
	"context"
	"net/http"
	"time"
)

func ContextMiddle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("Check")
		if cookie != nil {
			ctx := context.WithValue(r.Context(), "Check", cookie.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if chk := r.Context().Value("Check"); chk == "42" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("let us go\n"))
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not pass"))
	}
}

func CheckHandler(w http.ResponseWriter, r *http.Request) {
	exp := time.Now().Add(24 * time.Hour)
	cookie := http.Cookie{Name: "Check", Value: "42", Expires: exp}
	http.SetCookie(w, &cookie)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/chk", CheckHandler)

	ctxMux := ContextMiddle(mux)
	http.ListenAndServe(":8080", ctxMux)
}
