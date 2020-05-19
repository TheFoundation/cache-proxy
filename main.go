package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"
)

var (
	ttl time.Duration
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {

	logLevelAsString := os.Getenv("LOG_LEVEL")
	if logLevelAsString == "" {
		logLevelAsString = "info"
	}
	logLevel, err := log.ParseLevel(logLevelAsString)
	if err != nil {
		return err
	}
	log.SetLevel(logLevel)

	upstreamUrlAsString := os.Getenv("UPSTREAM_URL")
	if upstreamUrlAsString == "" {
		return fmt.Errorf("UPSTREAM_URL is required")
	}
	upstreamUrl, err := url.Parse(upstreamUrlAsString)
	if err != nil {
		return err
	}

	attemptHTTP2AsString := os.Getenv("ATTEMPT_HTTP2")
	if attemptHTTP2AsString == "" {
		attemptHTTP2AsString = "false"
	}
	attemptHTTP2, err := strconv.ParseBool(attemptHTTP2AsString)
	if err != nil {
		return err
	}

	ttlAsString := os.Getenv("TTL")
	if ttlAsString == "" {
		ttlAsString = "5m"
	}
	ttl, err = time.ParseDuration(ttlAsString)
	if err != nil {
		return err
	}

	frontendUrl := os.Getenv("FRONTEND_URL")
	if frontendUrl == "" {
		frontendUrl = ":8080"
	}

	backendUrlAsString := os.Getenv("BACKEND_URL")
	if backendUrlAsString == "" {
		backendUrlAsString = "badger://"
	}
	backendUrl, err := url.Parse(backendUrlAsString)
	if err != nil {
		return err
	}
	backend, err := newBackend(backendUrl)
	if err != nil {
		return err
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamUrl)
	proxy.Transport = &cachingTransport{
		upstream: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
			MaxIdleConns:          20,
			IdleConnTimeout:       120 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			ForceAttemptHTTP2:     attemptHTTP2,
		},
		cache: backend,
	}

	return http.ListenAndServe(frontendUrl, proxy)
}
