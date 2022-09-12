package serverPool

import (
	"github.com/simplelb/backend"
	"github.com/simplelb/common"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

const (
	ROUNDR = "round_robin"
	RANDOM = "random"
)

var Algmap = make(map[string]Next)

// ServerPool holds information about reachable backends
type ServerPool struct {
	backends []*backend.Backend
	current  uint64
	sel      Next
}

func NewServerPool(alg string) (*ServerPool, error) {
	tmp := &ServerPool{
		sel: Algmap[alg],
	}
	tmp.healthCheck()
	return tmp, nil
}

func (s *ServerPool) AddBackend(backend *backend.Backend) {
	s.backends = append(s.backends, backend)
	s.sel.LenInc()
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

func (s *ServerPool) LB(w http.ResponseWriter, r *http.Request) {
	attemps := common.GetAttemptsFromContext(r)
	if attemps > 3 {
		log.Printf("(%s)(%s) Max attempts reached,terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := s.GetNextPeer()
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
			atomic.StoreUint64(&s.current, uint64(idx))
			return s.backends[idx]
		}
	}
	return nil
}

func (s *ServerPool) healthCheck() {
	go func() {
		t := time.NewTicker(time.Minute * 2)
		for {
			select {
			case <-t.C:
				log.Printf("Starting health check..")
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
				log.Printf("Health check completed")
			}
		}
	}()
}
