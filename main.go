package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/simplelb/backend"
	"github.com/simplelb/common"
	serverPool2 "github.com/simplelb/serverPool"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func main() {
	var serverList string
	var port int
	var alg string
	flag.StringVar(&serverList, "backends", "", "Load balanced backends,use commas to separate")
	flag.IntVar(&port, "port", 3030, "Port to serve")
	flag.StringVar(&alg, "algorithm", "random", "selector algorithm")
	flag.Parse()

	if len(serverList) == 0 {
		log.Fatal("Please provide one or more backends to load balance")
	}

	serverPool, _ := serverPool2.NewServerPool(alg)
	tokens := strings.Split(serverList, ",")
	for _, tok := range tokens {
		serverUrl, err := url.Parse(tok)
		if err != nil {
			log.Fatal(err)
		}

		proxy := httputil.NewSingleHostReverseProxy(serverUrl)
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Printf("[%s] %s\n", serverUrl.Host, err.Error())
			retries := common.GetRetryFromContext(request)
			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(request.Context(), common.Retry, retries+1)
					proxy.ServeHTTP(writer, request.WithContext(ctx))
				}
				return
			}

			//after 3 retries, mark this backend as down
			serverPool.MarkBackendStatus(serverUrl, false)

			//QA what's the attempts mean?
			attempts := common.GetAttemptsFromContext(request)
			log.Printf("%s(%s) Attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), common.Attempts, attempts+1)
			serverPool.LB(writer, request.WithContext(ctx))
		}

		backen, _ := backend.NewBackend(serverUrl, proxy)
		serverPool.AddBackend(backen)
		log.Printf("Configured server: %s\n", serverUrl)
	}

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: http.HandlerFunc(serverPool.LB),
	}

	log.Printf("Load Balancer started at :%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
