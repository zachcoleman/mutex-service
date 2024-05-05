package main

import (
	"log"
	"net/http"
	"time"
)

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		end := time.Now()
		log.Println(r.Method, r.URL.Path, end.Sub(start))
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

func ApplyMiddlewares(h http.Handler) http.Handler {
	mws := []func(http.Handler) http.Handler{
		LoggerMiddleware,
		CORSMiddleware,
	}
	for _, middleware := range mws {
		h = middleware(h)
	}
	return h
}
