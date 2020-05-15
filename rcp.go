package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/go-redis/redis/v7"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

type cachingTransport struct {
	delegate    http.RoundTripper
	redisClient *redis.Client
	cachePrefix string
	cacheTTL    time.Duration
}

func (t *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {

	useCache := req.Method == "HEAD" || req.Method == "GET"

	cacheKey := fmt.Sprintf("%s:%s", t.cachePrefix, req.URL.String())

	if useCache {
		respDataBase64, err := t.redisClient.Get(cacheKey).Result()
		if err != nil {
			if err != redis.Nil {
				log.Warnf("unable to get %q: %v", cacheKey, err)
			}
		} else {
			respData, err := base64.StdEncoding.DecodeString(respDataBase64)
			if err != nil {
				log.Errorf("unable to decode response: %v", err)
			} else {
				resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(respData)), req)
				if err != nil {
					log.Errorf("unable to read response: %v", err)
				} else {
					log.Infof("HIT %s %s (%d)", req.Method, req.URL, resp.StatusCode)
					return resp, nil
				}
			}
		}
	}

	req.Host = req.URL.Host

	if log.IsLevelEnabled(log.DebugLevel) {
		if reqData, err := httputil.DumpRequest(req, log.IsLevelEnabled(log.TraceLevel)); err == nil {
			log.Debug(string(reqData))
		}
	}

	resp, err := t.delegate.RoundTrip(req)
	if err != nil {
		log.Errorf("unable to execute round trip to upstream: %v", err)
		return nil, err
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		if respData, err := httputil.DumpResponse(resp, log.IsLevelEnabled(log.TraceLevel)); err == nil {
			log.Debug(string(respData))
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("unable to read upstream body: %v", err)
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		log.Warnf("unable to close upstream body: %v", err)
		return nil, err
	}

	if useCache && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		go func() {
			resp.Body = ioutil.NopCloser(bytes.NewReader(body))
			respData := bytes.Buffer{}
			err := resp.Write(&respData)
			if err != nil {
				log.Errorf("unable to write response: %v", err)
			} else {
				respDataBase64 := base64.StdEncoding.EncodeToString(respData.Bytes())
				_, err = t.redisClient.Set(cacheKey, respDataBase64, t.cacheTTL).Result()
				if err != nil {
					log.Errorf("unable to set %q: %v", cacheKey, err)
				}
			}
		}()
	}

	log.Infof("MISS %s %s (%d)", req.Method, req.URL, resp.StatusCode)

	resp.Body = ioutil.NopCloser(bytes.NewReader(body))
	return resp, nil
}

func run() error {

	logLevelAsString := os.Getenv("RCP_LOG_LEVEL")
	if logLevelAsString == "" {
		logLevelAsString = "info"
	}
	logLevel, err := log.ParseLevel(logLevelAsString)
	if err != nil {
		return err
	}
	log.SetLevel(logLevel)

	upstreamUrlAsString := os.Getenv("RCP_UPSTREAM_URL")
	if upstreamUrlAsString == "" {
		return fmt.Errorf("RCP_UPSTREAM_URL is required")
	}
	upstreamUrl, err := url.Parse(upstreamUrlAsString)
	if err != nil {
		return err
	}

	redisUrl := os.Getenv("RCP_REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}

	cachePrefix := os.Getenv("RCP_CACHE_PREFIX")
	if cachePrefix == "" {
		cachePrefix = "rcp"
	}

	cacheTTLAsString := os.Getenv("RCP_CACHE_TTL")
	if cacheTTLAsString == "" {
		cacheTTLAsString = "5m"
	}
	cacheTTL, err := time.ParseDuration(cacheTTLAsString)
	if err != nil {
		return err
	}

	frontendUrl := os.Getenv("RCP_FRONTEND_URL")
	if frontendUrl == "" {
		frontendUrl = ":8080"
	}

	redisOptions, err := redis.ParseURL(redisUrl)
	if err != nil {
		return err
	}
	redisClient := redis.NewClient(redisOptions)

	proxy := httputil.NewSingleHostReverseProxy(upstreamUrl)
	proxy.Transport = &cachingTransport{
		delegate:    http.DefaultTransport,
		redisClient: redisClient,
		cachePrefix: cachePrefix,
		cacheTTL:    cacheTTL,
	}

	return http.ListenAndServe(frontendUrl, proxy)
}

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}
