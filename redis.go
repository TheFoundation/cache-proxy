package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/go-redis/redis/v7"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
)

type redisCache struct {
	client *redis.Client
}

func newRedisCache(url *url.URL) (*redisCache, error) {
	redisOptions, err := redis.ParseURL(url.String())
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(redisOptions)
	return &redisCache{client: client}, nil
}

func (c *redisCache) get(req *http.Request) (*http.Response, error) {
	key := "cache-proxy:" + req.URL.String()
	data, err := c.client.Get(key).Bytes()
	if err != nil {
		if err != redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get %q: %w", key, err)
	}
	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(data)), req)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %w", err)
	}
	return res, nil
}

func (c *redisCache) set(req *http.Request, res *http.Response) error {
	buf := bytes.Buffer{}
	err := res.Write(&buf)
	if err != nil {
		return fmt.Errorf("unable to write response to buffer: %w", err)
	}
	go func() {
		key := "cache-proxy:" + req.URL.String()
		err = c.client.Set(key, buf.Bytes(), ttl).Err()
		if err != nil {
			log.Warnf("unable to set %q: %v", key, err)
		}
	}()
	return nil
}
