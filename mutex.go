package main

import (
	"log"
	"net/http"
	"sync"
)

type MapMutex struct {
	Keys  map[string]struct{}
	RKeys map[string]uint
	Mut   sync.RWMutex
}

func LockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.URL.Query().Get("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// if not locked and not read-locked -> lock
		mmut.Mut.Lock()
		_, present := mmut.Keys[val]
		readers := mmut.RKeys[val]
		if !present && readers == 0 {
			mmut.Keys[val] = struct{}{}
		}
		mmut.Mut.Unlock()

		// if already locked or read-locked
		if present || readers > 0 {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func RLockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.URL.Query().Get("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// if not locked -> add read-lock
		mmut.Mut.Lock()
		_, wlock := mmut.Keys[val]
		if !wlock {
			mmut.RKeys[val] += 1
		}
		mmut.Mut.Unlock()

		// if already locked
		if wlock {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func UnlockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.URL.Query().Get("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// if locked -> unlock
		mmut.Mut.Lock()
		_, present := mmut.Keys[val]
		if present {
			delete(mmut.Keys, val)
		}
		mmut.Mut.Unlock()

		// if already unlocked
		if !present {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func RUnlockHandlerFactory(mmut *MapMutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.URL.Query().Get("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// if possible -> remove a read-lock
		// if not possible -> noop
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
		val := r.URL.Query().Get("key")
		if val == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// get "readable" status
		mmut.Mut.RLock()
		_, present := mmut.Keys[val]
		mmut.Mut.RUnlock()

		// if locked -> not readable
		if present {
			w.WriteHeader(http.StatusLocked)
			return
		}
		// if read-locked or no lock -> readable
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	mux := http.NewServeMux()
	mmut := MapMutex{
		Keys:  make(map[string]struct{}),
		RKeys: make(map[string]uint),
		Mut:   sync.RWMutex{},
	}
	// 404s for no "key" param value e.g. `?key=blah`
	// Note: Conflict can efficiently be translated to noop on read unlock
	mux.HandleFunc("GET /lock", LockHandlerFactory(&mmut))       // 202 (Accepted) or 409 (Conflict)
	mux.HandleFunc("GET /unlock", UnlockHandlerFactory(&mmut))   // 202 (Accepted) or 409 (Conflict)
	mux.HandleFunc("GET /rlock", RLockHandlerFactory(&mmut))     // 202 (Accepted) or 409 (Conflict)
	mux.HandleFunc("GET /runlock", RUnlockHandlerFactory(&mmut)) // 202 (Accepted)
	mux.HandleFunc("GET /status", StatusHandlerFactory(&mmut))   // 200 (OK) or 423 (Locked)
	handler := ApplyMiddlewares(mux)
	log.Fatal(http.ListenAndServe(":8080", handler))
}
