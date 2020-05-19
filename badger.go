package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/dgraph-io/badger/v2"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
)

type badgerCache struct {
	db *badger.DB
}

func newBadgerCache(url *url.URL) (*badgerCache, error) {
	path := url.Path
	if path == "" {
		path = "/tmp/cache-proxy"
	}
	opt := badger.DefaultOptions(path)
	opt.Logger = log.StandardLogger()
	db, err := badger.Open(opt)
	if err != nil {
		return nil, err
	}
	return &badgerCache{db: db}, nil
}

func (c *badgerCache) get(req *http.Request) (*http.Response, error) {
	var res *http.Response
	key := []byte("cache-proxy:" + req.URL.String())
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			return nil
		}
		return item.Value(func(val []byte) error {
			lres, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(val)), req)
			if err != nil {
				return fmt.Errorf("unable to read response: %w", err)
			}
			res = lres
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get %q: %w", string(key), err)
	}
	return res, err
}

func (c *badgerCache) set(req *http.Request, res *http.Response) error {
	buf := bytes.Buffer{}
	err := res.Write(&buf)
	if err != nil {
		return fmt.Errorf("unable to write response to buffer: %w", err)
	}
	key := []byte("cache-proxy:" + req.URL.String())
	go func() {
		err := c.db.Update(func(txn *badger.Txn) error {
			return txn.SetEntry(badger.NewEntry(key, buf.Bytes()).WithTTL(ttl))
		})
		if err != nil {
			log.Warnf("unable to set %q: %v", string(key), err)
		}
	}()
	return nil
}
