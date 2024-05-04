package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type MapMutex struct {
	Keys  map[string]struct{}
	RKeys map[string]uint
	Mut   sync.RWMutex
}

func LockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.PathValue("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("No valid key present"))
			return
		}

		// if no readers and not already locked, then lock
		mmut.Mut.Lock()
		_, present := mmut.Keys[val]
		readers := mmut.RKeys[val]
		if !present && readers == 0 {
			mmut.Keys[val] = struct{}{}
		}
		mmut.Mut.Unlock()

		// if locked or read locks
		if present || readers > 0 {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(fmt.Sprintf("Key: %s already locked", val)))
			return
		}

		// else
		w.WriteHeader(http.StatusAccepted)
	}
}

func RLockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.PathValue("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("No valid key present"))
			return
		}

		// if no lock, increase reader locks
		mmut.Mut.Lock()
		_, wlock := mmut.Keys[val]
		if !wlock {
			mmut.RKeys[val] += 1
		}
		mmut.Mut.Unlock()

		// if locked can't rlock
		if wlock {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(fmt.Sprintf("Key: %s locked", val)))
			return
		}
		// else
		w.WriteHeader(http.StatusAccepted)
	}
}

func UnlockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.PathValue("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("No valid key present"))
			return
		}

		// release lock if present
		mmut.Mut.Lock()
		_, present := mmut.Keys[val]
		if present {
			delete(mmut.Keys, val)
		}
		mmut.Mut.Unlock()

		// if unlocked
		if !present {
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(fmt.Sprintf("Key: %s already unlocked", val)))
			return
		}
		// else
		w.WriteHeader(http.StatusAccepted)
	}
}

func RUnlockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.PathValue("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("No valid key present"))
			return
		}

		// release a reader lock if present
		mmut.Mut.Lock()
		readers := mmut.RKeys[val]
		if readers > 0 {
			mmut.RKeys[val] -= 1
		}
		mmut.Mut.Unlock()

		w.WriteHeader(http.StatusAccepted)
	}
}

func StatusHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.PathValue("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("No valid key present"))
			return
		}

		// acquire exclusive access, read, and/or perform ops then release
		mmut.Mut.RLock()
		_, present := mmut.Keys[val]
		mmut.Mut.RUnlock()

		// if locked
		if present {
			w.WriteHeader(http.StatusLocked)
			w.Write([]byte(fmt.Sprintf("Key: %s locked", val)))
			return
		}
		// if rlocked or unlocked
		w.WriteHeader(http.StatusOK)
	}
}

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

func main() {
	mux := http.NewServeMux()
	mmut := MapMutex{
		Keys:  make(map[string]struct{}),
		RKeys: make(map[string]uint),
		Mut:   sync.RWMutex{},
	}
	mux.HandleFunc("GET /lock/{key}", LockHandlerFactory(&mmut))
	mux.HandleFunc("GET /unlock/{key}", UnlockHandlerFactory(&mmut))
	mux.HandleFunc("GET /rlock/{key}", RLockHandlerFactory(&mmut))
	mux.HandleFunc("GET /runlock/{key}", RUnlockHandlerFactory(&mmut))
	mux.HandleFunc("GET /status/{key}", StatusHandlerFactory(&mmut))
	handler := ApplyMiddlewares(mux)
	log.Fatal(http.ListenAndServe(":8080", handler))
}
