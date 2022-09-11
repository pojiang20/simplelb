package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/simplelb/backend"
	"github.com/simplelb/selector"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const (
	Attempts int = iota
	Retry
)

// ServerPool holds information about reachable backends
type ServerPool struct {
	backends []*backend.Backend
	current  uint64
	sel selector.Next
}

func NewServerPool() (*ServerPool,error) {
	tmp := &ServerPool{
		sel: selector.RoundR{}
	}
	return tmp,nil
}

func (s *ServerPool) AddBackend(backend *backend.Backend) {
	s.backends = append(s.backends, backend)
}

// changes a status of a backend
func (s *ServerPool) MarkBackendStatus(backendUrl *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.URL.String() == backendUrl.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("Site unreachable,error: ", err)
		return false
	}
	defer conn.Close()
	return true
}

func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		status := "up"
		//send tcp connection to check alive
		alive := isBackendAlive(b.URL)
		b.SetAlive(alive)
		if !alive {
			status = "down"
		}
		log.Printf("%s [%s]", b.URL, status)
	}
}

func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}
	return 1
}

func GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func lb(w http.ResponseWriter, r *http.Request) {
	attemps := GetAttemptsFromContext(r)
	if attemps > 3 {
		log.Printf("(%s)(%s) Max attempts reached,terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := serverPool.GetNextPeer()
	if peer != nil {
		log.Printf("change server to peer: %s", peer.URL)
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}

// returns selector active peer to take a connection
func (s *ServerPool) GetNextPeer() *backend.Backend {
	next := s.sel.NextIndex()
	length := len(s.backends)
	for i := 0; i < length; i++ {
		idx := (next + i) % length //start from "selector" and move a full cycle length
		if s.backends[idx].IsAlive() {
			if idx != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}
			return s.backends[idx]
		}
	}
	return nil
}

func healthCheck() {
	go func() {
		t := time.NewTicker(time.Minute * 2)
		for {
			select {
			case <-t.C:
				log.Printf("Starting health check..")
				serverPool.HealthCheck()
				log.Printf("Health check completed")
			}
		}
	}()
}

var serverPool ServerPool

func main() {
	var serverList string
	var port int
	flag.StringVar(&serverList, "backends", "", "Load balanced backends,use commas to separate")
	flag.IntVar(&port, "port", 3030, "Port to serve")
	flag.Parse()

	if len(serverList) == 0 {
		log.Fatal("Please provide one or more backends to load balance")
	}

	tokens := strings.Split(serverList, ",")
	for _, tok := range tokens {
		serverUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatal(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Printf("[%s] %s\n", serverUrl.Host, err.Error())
			retries := GetRetryFromContext(request)
			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(request.Context(), Retry, retries+1)
					proxy.ServeHTTP(writer, request.WithContext(ctx))
				}
				return
			}

			//after 3 retries, mark this backend as down
			serverPool.MarkBackendStatus(serverUrl, false)

			//QA what's the attempts mean?
			attempts := GetAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), Attempts, attempts+1)
			lb(writer, request.WithContext(ctx))
		}

		backend, _ := backend.NewBackend(serverUrl, proxy)
		serverPool.AddBackend(backend)
		log.Printf("Configured server: %s\n", serverUrl)
	}

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(lb),
	}

	healthCheck()

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
