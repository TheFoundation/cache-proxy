package main

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type cacher interface {
	get(*http.Request) (*http.Response, error)
	set(*http.Request, *http.Response) error
}

type cachingTransport struct {
	upstream http.RoundTripper
	cache    cacher
}

func (t *cachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {

	start := time.Now()

	req.Host = req.URL.Host

	useCache := req.Method == "HEAD" || req.Method == "GET"

	if useCache {
		res, err := t.cache.get(req)
		if err != nil {
			log.Errorf("unable to query cache: %v", err)
			err = nil
		} else if res != nil {
			log.Infof("HIT %s %s (%d) %s", req.Method, req.URL, res.StatusCode, time.Now().Sub(start))
			return res, nil
		}
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		if reqData, err := httputil.DumpRequest(req, log.IsLevelEnabled(log.TraceLevel)); err == nil {
			log.Debug(string(reqData))
		}
	}

	res, err := t.upstream.RoundTrip(req)
	if err != nil {
		if useCache {
			log.Warnf("first try: unable to execute round trip to upstream: %v", err)
			err = nil
			time.Sleep(1 * time.Second)
			res, err = t.upstream.RoundTrip(req)
		}
		if err != nil {
			log.Errorf("unable to execute round trip to upstream: %v", err)
			return nil, err
		}
	}

	if log.IsLevelEnabled(log.DebugLevel) {
		if respData, err := httputil.DumpResponse(res, log.IsLevelEnabled(log.TraceLevel)); err == nil {
			log.Debug(string(respData))
		}
	}

	var body []byte
	if res.Body != nil {
		body, err = ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorf("unable to read upstream body: %v", err)
			return nil, err
		}
		err = res.Body.Close()
		if err != nil {
			log.Warnf("unable to close upstream body: %v", err)
			err = nil
		}
	}

	if useCache && res.StatusCode >= 200 && res.StatusCode < 300 {
		if body != nil {
			res.Body = ioutil.NopCloser(bytes.NewReader(body))
		}
		err = t.cache.set(req, res)
		if err != nil {
			log.Errorf("unable to update cache: %v", err)
			err = nil
		}
	}

	if body != nil {
		res.Body = ioutil.NopCloser(bytes.NewReader(body))
	}

	log.Infof("MISS %s %s (%d) %s", req.Method, req.URL, res.StatusCode, time.Now().Sub(start))

	return res, nil
}

func newBackend(url *url.URL) (cacher, error) {
	switch url.Scheme {
	case "redis":
		return newRedisCache(url)
	case "badger":
		return newBadgerCache(url)
	}
	return nil, fmt.Errorf("invalid cache backend %q", url.Scheme)
}
