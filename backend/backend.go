package backend

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

// Backend holds the data about a server
type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

func NewBackend(serverUrl *url.URL, proxy *httputil.ReverseProxy) (*Backend, error) {
	tmp := &Backend{
		URL:          serverUrl,
		Alive:        true,
		ReverseProxy: proxy,
	}
	tmp.StartServer()
	return tmp, nil
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.Alive
	b.mux.RUnlock()
	return
}

func (b *Backend) StartServer() {
	go func() {
		port := b.URL.Port()
		log.Printf("[%s] start", port)
		http.HandleFunc("/"+port, HelloHandler)
		http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	}()
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello World:%s", r.URL)
}
