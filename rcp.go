package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/go-redis/redis/v7"
	"io/ioutil"
	"log"
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
				log.Printf("unable to get %q: %v", cacheKey, err)
			}
		} else {
			respData, err := base64.StdEncoding.DecodeString(respDataBase64)
			if err != nil {
				log.Printf("unable to decode response: %v", err)
			} else {
				resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(respData)), req)
				if err != nil {
					log.Printf("unable to read response: %v", err)
				} else {
					log.Printf("HIT %s %s (%d)", req.Method, req.URL, resp.StatusCode)
					return resp, nil
				}
			}
		}
	}

	req.Host = req.URL.Host

	//reqData, err := httputil.DumpRequest(req, true)
	//if err == nil {
	//	log.Println(string(reqData))
	//}

	resp, err := t.delegate.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	//respData, err := httputil.DumpResponse(resp, true)
	//if err == nil {
	//	log.Println(string(respData))
	//}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	//noinspection GoUnhandledErrorResult
	defer resp.Body.Close()

	resp.Body = ioutil.NopCloser(bytes.NewReader(body))

	log.Printf("MIS %s %s (%d)", req.Method, req.URL, resp.StatusCode)

	if useCache && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		respData := bytes.Buffer{}
		err := resp.Write(&respData)
		if err != nil {
			log.Printf("unable to write response: %v", err)
		} else {
			go func() {
				respDataBase64 := base64.StdEncoding.EncodeToString(respData.Bytes())
				_, err = t.redisClient.Set(cacheKey, respDataBase64, t.cacheTTL).Result()
				if err != nil {
					log.Printf("unable to set %q: %v", cacheKey, err)
				}
			}()
		}
	}

	return resp, nil
}

func run() error {

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
		log.Fatal(err)
	}
}
